/** Clear OAuth return query params from a URL (mutates in place). */
export function clearAuthCallbackParams(url: URL) {
  url.searchParams.delete("googleAuth");
  url.searchParams.delete("googleAuthError");
  url.searchParams.delete("auth");
  url.searchParams.delete("authError");
  url.searchParams.delete("provider");
}

export function readAuthCallback(url: URL): {
  kind: "success" | "error" | null;
  errorMessage: string;
} {
  const googleAuth = url.searchParams.get("googleAuth");
  const genericAuth = url.searchParams.get("auth");
  if (googleAuth === "success" || genericAuth === "success") {
    return { kind: "success", errorMessage: "" };
  }
  if (googleAuth === "error" || genericAuth === "error") {
    return {
      kind: "error",
      errorMessage:
        url.searchParams.get("googleAuthError") ||
        url.searchParams.get("authError") ||
        "Login failed",
    };
  }
  return { kind: null, errorMessage: "" };
}
