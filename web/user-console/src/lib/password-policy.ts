// Mirrors the control-plane password policy (RB§9): keep both sides in sync.
export const PASSWORD_MIN_LENGTH = 8;

export function passwordIssue(password: string): string | null {
  if (password.length < PASSWORD_MIN_LENGTH) {
    return `password must be at least ${PASSWORD_MIN_LENGTH} characters`;
  }
  if (!/[a-zA-Z]/.test(password) || !/[0-9]/.test(password)) {
    return "password must contain letters and numbers";
  }
  return null;
}
