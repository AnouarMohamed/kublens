import type { MutableRefObject } from "react";
import { parseStreamEvent } from "../../../lib/api/stream";
import type { K8sEvent } from "../../../types";
import { redactSensitiveText } from "../../../lib/security/redaction";

export interface NotificationSignalMetrics {
  totalLast5Minutes: number;
  warningLast10Minutes: number;
  burstDetected: boolean;
}

export function parseWSStreamPayload<T>(data: string): { type: string; timestamp: string; payload: T } | null {
  return parseStreamEvent<T>(data);
}

export function buildEventKey(event: K8sEvent): string {
  return [
    event.type ?? "",
    event.reason ?? "",
    event.from ?? "",
    event.message ?? "",
    event.lastTimestamp ?? event.age ?? "",
  ].join("|");
}

export function maybeSendDesktopNotification(event: K8sEvent, permissionRequestedRef: MutableRefObject<boolean>): void {
  if (typeof window === "undefined" || typeof Notification === "undefined") {
    return;
  }

  if (Notification.permission === "granted") {
    const title = event.reason ? `KubeLens: ${event.reason}` : "KubeLens event";
    const body = event.message ? event.message.slice(0, 180) : "Cluster event received.";
    // Browser-level notifications are best effort; errors are intentionally ignored.
    try {
      new Notification(title, { body });
    } catch {
      // no-op
    }
    return;
  }

  if (Notification.permission === "default" && !permissionRequestedRef.current) {
    permissionRequestedRef.current = true;
    void Notification.requestPermission();
  }
}

export function keywordSetFromSignature(signature: string): Set<string> {
  if (signature.trim() === "") {
    return new Set<string>();
  }
  const out = new Set<string>();
  for (const keyword of signature.split("|")) {
    const normalized = keyword.trim().toLowerCase();
    if (normalized !== "") {
      out.add(normalized);
    }
  }
  return out;
}

export function matchesMutedKeyword(event: K8sEvent, mutedKeywords: Set<string>): boolean {
  if (mutedKeywords.size === 0) {
    return false;
  }
  const haystack = `${event.reason} ${event.message} ${event.from} ${event.type}`.toLowerCase();
  for (const keyword of mutedKeywords) {
    if (haystack.includes(keyword)) {
      return true;
    }
  }
  return false;
}

export function redactEventFields(event: K8sEvent): K8sEvent {
  return {
    ...event,
    reason: redactSensitiveText(event.reason),
    message: redactSensitiveText(event.message),
    from: redactSensitiveText(event.from),
  };
}

export function deriveNotificationSignal(events: K8sEvent[], burstThreshold: number): NotificationSignalMetrics {
  const now = Date.now();
  let totalLast5Minutes = 0;
  let warningLast10Minutes = 0;

  for (const event of events) {
    const timestamp = parseTimestampMs(event.lastTimestamp);
    if (timestamp === 0) {
      continue;
    }
    const ageMs = now - timestamp;
    if (ageMs <= 5 * 60 * 1000) {
      totalLast5Minutes += 1;
    }
    if (ageMs <= 10 * 60 * 1000 && notificationTone(event.type) === "warning") {
      warningLast10Minutes += 1;
    }
  }

  const threshold = Math.max(3, burstThreshold);
  return {
    totalLast5Minutes,
    warningLast10Minutes,
    burstDetected: warningLast10Minutes >= threshold,
  };
}

function notificationTone(type: string): "warning" | "normal" | "other" {
  const normalized = type.trim().toLowerCase();
  if (normalized === "warning") {
    return "warning";
  }
  if (normalized === "normal") {
    return "normal";
  }
  return "other";
}

function parseTimestampMs(value?: string): number {
  if (!value) {
    return 0;
  }
  const parsed = Date.parse(value);
  if (Number.isNaN(parsed)) {
    return 0;
  }
  return parsed;
}
