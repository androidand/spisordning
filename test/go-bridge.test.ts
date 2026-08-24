import { describe, test, expect } from "bun:test";
import { execFileSync } from "node:child_process";
import { existsSync } from "node:fs";
import { resolve } from "node:path";

const repoRoot = resolve(import.meta.dir, "..");

describe("spisordning Go test bridge", () => {
  test("repo is a Go module", () => {
    expect(existsSync(resolve(repoRoot, "go.mod"))).toBe(true);
  });

  test("go test ./... passes", () => {
    expect(() =>
      execFileSync("go", ["test", "./..."], {
        cwd: repoRoot,
        encoding: "utf-8",
      })
    ).not.toThrow();
  });
});
