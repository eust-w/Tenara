package build

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	cloneImage       = "alpine/git:v2.49.1"
	tokenMountPath   = "/var/run/tenara-git" //nolint:gosec // G101 false positive: volume mount path, not a credential
	tokenFileName    = "token"
	workspaceVolume  = "workspace"
	workspaceMount   = "/workspace"
	workspaceSrcPath = "/workspace/src"
)

// EphemeralTokenSecretName derives the deterministic short-lived Secret name
// holding the GitHub token for one build.
func EphemeralTokenSecretName(buildName string) string {
	return buildName + "-git-token"
}

// EphemeralTokenSecret builds the per-build Secret carrying the GitHub token.
// It is owned by the Build so garbage collection cannot leak it.
func EphemeralTokenSecret(b *Build, token []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      EphemeralTokenSecretName(b.Name),
			Namespace: b.Namespace,
			Labels: map[string]string{
				"tenara.io/build":     b.Name,
				"tenara.io/ephemeral": "true",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         APIVersion,
				Kind:               KindBuild,
				Name:               b.Name,
				UID:                b.UID,
				Controller:         boolPtr(true),
				BlockOwnerDeletion: boolPtr(true),
			}},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{tokenFileName: token},
	}
}

// CloneInitContainer returns the init container performing a shallow clone of
// the pinned SHA into the shared workspace emptyDir. The GitHub token never
// enters environment variables, logs, or image layers: the credential helper
// reads it from the read-only mounted Secret volume at helper invocation time.
func CloneInitContainer(b *Build) corev1.Container {
	script := "set -ec; mkdir -p " + workspaceSrcPath + "; " +
		"git init -q " + workspaceSrcPath + "; " +
		"git -C " + workspaceSrcPath + " remote add origin \"$GIT_URL\"; " +
		"git -C " + workspaceSrcPath + " -c credential.helper='" + credentialHelperSnippet() + "'" +
		" fetch --depth 1 origin \"$GIT_SHA\"; " +
		"git -C " + workspaceSrcPath + " checkout -q --detach FETCH_HEAD"

	return corev1.Container{
		Name:            "git-clone",
		Image:           cloneImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"sh", "-ec", script},
		Env: []corev1.EnvVar{
			{Name: "GIT_URL", Value: b.Spec.Git.URL},
			{Name: "GIT_SHA", Value: b.Spec.Git.SHA},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      EphemeralTokenSecretName(b.Name),
				MountPath: tokenMountPath,
				ReadOnly:  true,
			},
			{
				Name:      workspaceVolume,
				MountPath: workspaceMount,
			},
		},
	}
}

func credentialHelperSnippet() string {
	return "!f() { printf \"username=x-access-token\\npassword=%s\\n\" \"$(cat " +
		tokenMountPath + "/" + tokenFileName + ")\"; }; f"
}

// AppendCloneStage wires the clone stage into the builder Pod: token volume,
// shared workspace emptyDir, the init container itself, and the matching
// workspace mount on the buildkitd container for subsequent build steps.
func AppendCloneStage(pod *corev1.Pod, b *Build) {
	tokenVolName := EphemeralTokenSecretName(b.Name)
	pod.Spec.Volumes = append(pod.Spec.Volumes,
		corev1.Volume{
			Name: tokenVolName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  tokenVolName,
					DefaultMode: int32Ptr(0o400),
				},
			},
		},
		corev1.Volume{
			Name: workspaceVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	)
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, CloneInitContainer(b))

	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == "buildkitd" {
			pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts,
				corev1.VolumeMount{Name: workspaceVolume, MountPath: workspaceMount})
		}
	}
}

// DeleteEphemeralTokenSecret removes the short-lived token Secret once the
// build has moved past CLONING. Missing Secrets are tolerated.
func DeleteEphemeralTokenSecret(ctx context.Context, c client.Client, namespace, buildName string) {
	key := types.NamespacedName{Namespace: namespace, Name: EphemeralTokenSecretName(buildName)}
	var s corev1.Secret
	if err := c.Get(ctx, key, &s); err != nil {
		return
	}
	if err := c.Delete(ctx, &s); err != nil {
		ctrl.Log.Error(err, "delete ephemeral git token secret", "secret", key.Name)
	}
}
