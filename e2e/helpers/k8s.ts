import { exec } from "node:child_process";
import { promisify } from "node:util";

const run = promisify(exec);

export async function k8s(args: string): Promise<string> {
  const { stdout } = await run(`kubectl ${args}`);
  return stdout.trim();
}

export function k8sJson<T>(args: string): Promise<T> {
  return k8s(`${args} -o json`).then((s) => JSON.parse(s) as T);
}
