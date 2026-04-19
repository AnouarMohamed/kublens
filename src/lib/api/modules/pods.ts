import type { ActionResult, K8sEvent, Pod, PodCreateRequest, PodDetail } from "../../../types";
import { apiRoute, requestJson, requestText } from "../core";

function buildPodLogQuery(tailLines: number, container?: string, follow?: boolean): string {
  const params = new URLSearchParams();
  if (tailLines > 0) {
    params.set("tailLines", String(tailLines));
  }
  if (container && container.trim() !== "") {
    params.set("container", container.trim());
  }
  if (typeof follow === "boolean") {
    params.set("follow", String(follow));
  }
  return params.toString();
}

export const podsApi = {
  getEvents: (signal?: AbortSignal) => requestJson<K8sEvent[]>(apiRoute("/events"), { signal }),
  getPods: (signal?: AbortSignal) => requestJson<Pod[]>(apiRoute("/pods"), { signal }),
  createPod: (payload: PodCreateRequest) =>
    requestJson<ActionResult>(apiRoute("/pods"), {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  getPodDetail: (namespace: string, name: string) =>
    requestJson<PodDetail>(apiRoute("/pods/{namespace}/{name}", { namespace, name })),
  getPodEvents: (namespace: string, name: string) =>
    requestJson<K8sEvent[]>(apiRoute("/pods/{namespace}/{name}/events", { namespace, name })),
  getPodLogs: (namespace: string, name: string, lines = 100, container?: string) => {
    const params = new URLSearchParams();
    if (lines > 0) {
      params.set("lines", String(lines));
    }
    if (container && container.trim() !== "") {
      params.set("container", container.trim());
    }
    const suffix = params.toString();
    return requestText(
      `${apiRoute("/pods/{namespace}/{name}/logs", { namespace, name })}${suffix ? `?${suffix}` : ""}`,
    );
  },
  getPodDescribe: (namespace: string, name: string) =>
    requestText(apiRoute("/pods/{namespace}/{name}/describe", { namespace, name })),
  getPodLogStreamURL: (namespace: string, name: string, tailLines = 100, container?: string, follow = true) => {
    const suffix = buildPodLogQuery(tailLines, container, follow);
    return `${apiRoute("/pods/{namespace}/{name}/logs/stream", { namespace, name })}${suffix ? `?${suffix}` : ""}`;
  },
  restartPod: (namespace: string, name: string) =>
    requestJson<ActionResult>(apiRoute("/pods/{namespace}/{name}/restart", { namespace, name }), {
      method: "POST",
    }),
  deletePod: (namespace: string, name: string) =>
    requestJson<ActionResult>(apiRoute("/pods/{namespace}/{name}", { namespace, name }), {
      method: "DELETE",
    }),
};
