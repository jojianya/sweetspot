import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
    // Vendored third-party bundles — not our code, and linting them
    // produces ~1098 warnings that drown out real issues.
    "public/**",
  ]),
  // Dependency-flow guard: pure lib must not import the store. The 401
  // session backstop is injected via setUnauthorizedHandler (registered by
  // SessionSync), so no allowlist remains. Test files are excluded: they
  // need the store to assert behavior. Hooks/components importing the
  // store is the correct direction and stays allowed.
  {
    files: ["src/shared/api/**", "src/shared/types/**"],
    ignores: ["**/*.test.{ts,tsx}"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["@/features/auth/store/*"],
              message:
                "Pure lib must not import the store. Inject a callback instead.",
            },
          ],
        },
      ],
    },
  },
  // Feature boundary: a feature's internals are private. Other features and
  // app routes may only use the public surface — the feature's index.ts —
  // never a deep path. Test files are excluded: vi.mock must target the
  // leaf module, because mocking a barrel would blank out every sibling
  // export the module under test needs.
  {
    files: ["src/features/**", "src/app/**", "src/shared/layout/**"],
    ignores: ["**/*.test.{ts,tsx}"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: [
                "@/features/*/*",
                "@/features/*/*/*",
                "@/features/*/*/**",
              ],
              message:
                "Import the feature's public surface (@/features/<name>) instead of a deep path. Mocks are exempt: keep mocking the leaf module.",
            },
          ],
        },
      ],
    },
  },
  // Shared library code must not depend on features at all. src/shared/layout
  // is intentionally outside this block: the app chrome (Navbar) renders
  // session-dependent UI and consumes the auth public surface. Test files are
  // excluded, as with the guards above.
  {
    files: [
      "src/shared/api/**",
      "src/shared/utils/**",
      "src/shared/lib/**",
      "src/shared/types/**",
      "src/shared/ui/**",
      "src/shared/hooks/**",
    ],
    ignores: ["**/*.test.{ts,tsx}"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["@/features/*", "@/features/*/**"],
              message:
                "Shared library code must not import from features. Move the dependency into shared/ or lift it to the caller.",
            },
          ],
        },
      ],
    },
  },
]);

export default eslintConfig;
