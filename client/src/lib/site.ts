const DEFAULT_LOCAL_SITE_URL = "http://localhost:3000";

function isLoopbackHostname(hostname: string): boolean {
  const normalized = hostname.toLowerCase();
  if (normalized === "localhost" || normalized.endsWith(".localhost")) return true;
  if (normalized === "::1" || normalized === "[::1]") return true;
  return /^127(?:\.\d{1,3}){3}$/.test(normalized);
}

function isPrivateHostname(hostname: string): boolean {
  const ip = hostname.replace(/^\[|\]$/g, ""); // strip IPv6 brackets
  const parts = ip.split(".").map(Number);
  if (parts.length !== 4 || parts.some((p) => isNaN(p))) return false;
  // 10.0.0.0/8
  if (parts[0] === 10) return true;
  // 172.16.0.0/12
  if (parts[0] === 172 && parts[1] >= 16 && parts[1] <= 31) return true;
  // 192.168.0.0/16
  if (parts[0] === 192 && parts[1] === 168) return true;
  return false;
}

function parseOrigin(raw: string, variableName: string): URL {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`${variableName} must be an absolute URL`);
  }

  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new Error(`${variableName} must use http or https`);
  }
  if (url.username || url.password || url.search || url.hash) {
    throw new Error(`${variableName} must be an origin without credentials, query, or hash`);
  }
  if (url.pathname !== "/") {
    throw new Error(`${variableName} must not include a path`);
  }
  if (url.protocol !== "https:" && !isLoopbackHostname(url.hostname) && !isPrivateHostname(url.hostname)) {
    throw new Error(`${variableName} must use https unless it is a local or private address`);
  }

  return new URL(url.origin);
}

/**
 * Resolves the public frontend origin used for canonical URLs and metadata.
 * Local development may omit SITE_URL, but a public API origin requires an
 * explicit SITE_URL so a production build cannot silently publish localhost
 * metadata.
 */
export function resolveSiteUrl(
  siteUrl = process.env.SITE_URL,
  apiUrl = process.env.NEXT_PUBLIC_API_URL
): URL {
  const configuredSiteUrl = siteUrl?.trim();
  if (configuredSiteUrl) return parseOrigin(configuredSiteUrl, "SITE_URL");

  const configuredApiUrl = apiUrl?.trim();
  if (configuredApiUrl) {
    let parsedApiUrl: URL;
    try {
      parsedApiUrl = new URL(configuredApiUrl);
    } catch {
      throw new Error("NEXT_PUBLIC_API_URL must be an absolute URL when SITE_URL is omitted");
    }
    if (!isLoopbackHostname(parsedApiUrl.hostname) && !isPrivateHostname(parsedApiUrl.hostname)) {
      throw new Error("SITE_URL is required when NEXT_PUBLIC_API_URL is not a local or private address");
    }
  }

  return new URL(DEFAULT_LOCAL_SITE_URL);
}

export function getSiteUrl(): URL {
  return resolveSiteUrl();
}
