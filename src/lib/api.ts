/**
 * Typed API facade used by all frontend views and hooks.
 *
 * Methods are grouped into domain modules under `src/lib/api/modules`.
 */
import { alertsApi } from "./api/modules/alerts";
import { assistantApi } from "./api/modules/assistant";
import { authApi } from "./api/modules/auth";
import { ghostApi } from "./api/modules/ghost";
import { incidentsApi } from "./api/modules/incidents";
import { nodesApi } from "./api/modules/nodes";
import { podsApi } from "./api/modules/pods";
import { remediationApi } from "./api/modules/remediation";
import { resourcesApi } from "./api/modules/resources";
import { systemApi } from "./api/modules/system";

export const api = {
  ...authApi,
  ...systemApi,
  ...alertsApi,
  ...resourcesApi,
  ...podsApi,
  ...nodesApi,
  ...ghostApi,
  ...assistantApi,
  ...incidentsApi,
  ...remediationApi,
};

export { ApiError } from "./api/core";
