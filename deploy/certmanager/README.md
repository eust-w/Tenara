# cert-manager integration (todo89 / D2-P2-2)

Flow: custom domain passes TXT verification (`POST /domains/{id}/verify`)
-> `domaincert.RenderCertificate` manifest is applied by the data plane ->
cert-manager completes ACME HTTP-01 -> `tls-<domainID>` secret feeds Envoy
Gateway routing.

Live gates (per design doc): cert-manager installed on the target cluster
and reachable ACME endpoint. `clusterissuer.yaml` ships the staging issuer
by default; production flip is an ops decision, not a code path.
