const LONG_SECRET_PATTERN = /\b[A-Za-z0-9+/_-]{24,}\b/g;
const KEY_VALUE_SECRET_PATTERN = /((?:token|password|secret|api[_-]?key)\s*[:=]\s*)[^\s,;]+/gi;
const BEARER_PATTERN = /(bearer\s+)[^\s]+/gi;

export function redactSensitiveText(value: string): string {
  if (!value) {
    return value;
  }

  return value
    .replace(BEARER_PATTERN, "$1[redacted]")
    .replace(KEY_VALUE_SECRET_PATTERN, "$1[redacted]")
    .replace(LONG_SECRET_PATTERN, "[redacted]");
}
