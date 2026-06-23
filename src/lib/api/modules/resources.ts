import type {
  ActionResult,
  ApplyResourceYAMLResponse,
  ResourceList,
  ResourceManifest,
  ScaleRequest,
} from "../../../types";
import { apiRoute, requestJson } from "../core";

export const resourcesApi = {
  getNamespaces: (signal?: AbortSignal) => requestJson<string[]>(apiRoute("/namespaces"), { signal }),
  getResources: (kind: string, signal?: AbortSignal) =>
    requestJson<ResourceList>(apiRoute("/resources/{kind}", { kind }), { signal }),
  getResourceYAML: (kind: string, namespace: string, name: string) =>
    requestJson<ResourceManifest>(apiRoute("/resources/{kind}/{namespace}/{name}/yaml", { kind, namespace, name })),
  applyResourceYAML: (kind: string, namespace: string, name: string, payload: ResourceManifest) =>
    requestJson<ApplyResourceYAMLResponse>(
      apiRoute("/resources/{kind}/{namespace}/{name}/yaml", { kind, namespace, name }),
      {
        method: "PUT",
        body: JSON.stringify(payload),
      },
    ),
  applyResourceYAMLWithForce: (
    kind: string,
    namespace: string,
    name: string,
    payload: ResourceManifest,
    force: boolean,
  ) =>
    requestJson<ApplyResourceYAMLResponse>(
      `${apiRoute("/resources/{kind}/{namespace}/{name}/yaml", { kind, namespace, name })}${force ? "?force=true" : ""}`,
      {
        method: "PUT",
        body: JSON.stringify(payload),
      },
    ),
  scaleResource: (kind: string, namespace: string, name: string, payload: ScaleRequest) =>
    requestJson<ActionResult>(apiRoute("/resources/{kind}/{namespace}/{name}/scale", { kind, namespace, name }), {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  restartResource: (kind: string, namespace: string, name: string) =>
    requestJson<ActionResult>(apiRoute("/resources/{kind}/{namespace}/{name}/restart", { kind, namespace, name }), {
      method: "POST",
    }),
  rollbackResource: (kind: string, namespace: string, name: string) =>
    requestJson<ActionResult>(apiRoute("/resources/{kind}/{namespace}/{name}/rollback", { kind, namespace, name }), {
      method: "POST",
    }),
};
