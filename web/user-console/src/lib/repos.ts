// Normalisation + search helpers for the GitHub repo picker (D5). Unknown
// upstream fields (including any token material) are structurally dropped.
export interface RepoItem {
  fullName: string;
  privateRepo: boolean;
  defaultBranch: string;
}

const ALLOWED_KEYS = new Set(["fullName", "privateRepo", "defaultBranch"]);

export function normalizeRepoList(payload: unknown): RepoItem[] {
  if (!Array.isArray(payload)) {
    return [];
  }
  const out: RepoItem[] = [];
  for (const raw of payload) {
    const record = raw as Record<string, unknown>;
    const fullName = typeof record.full_name === "string" ? record.full_name : "";
    if (fullName === "") {
      continue;
    }
    out.push({
      fullName,
      privateRepo: record.private === true,
      defaultBranch: typeof record.default_branch === "string" ? record.default_branch : "main",
    });
  }
  return out;
}

export function filterRepos(repos: RepoItem[], query: string): RepoItem[] {
  const q = query.trim().toLowerCase();
  if (q === "") {
    return repos;
  }
  return repos.filter((r) => r.fullName.toLowerCase().includes(q));
}

export function onlyPickerFields(item: RepoItem): Record<string, string | boolean> {
  return Object.fromEntries(
    Object.entries({
      fullName: item.fullName,
      privateRepo: item.privateRepo,
      defaultBranch: item.defaultBranch,
    }).filter(([k]) => ALLOWED_KEYS.has(k)),
  );
}
