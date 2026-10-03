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
    files: ["src/lib/api/**", "src/lib/types/**"],
    ignores: ["**/*.test.{ts,tsx}"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["@/store/*"],
              message:
                "Pure lib must not import the store. Inject a callback instead.",
            },
          ],
        },
      ],
    },
  },
]);

export default eslintConfig;
