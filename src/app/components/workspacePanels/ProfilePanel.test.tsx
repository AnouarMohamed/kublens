import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { AuthSession, RuntimeStatus } from "../../../types";
import { ProfilePanel } from "./ProfilePanel";

function runtimeFixture(): RuntimeStatus {
  return {
    mode: "prod",
    devMode: false,
    insecure: false,
    isRealCluster: true,
    authEnabled: true,
    writeActionsEnabled: false,
    predictorEnabled: true,
    predictorHealthy: true,
    ghostEnabled: true,
    ghostHealthy: true,
    assistantEnabled: true,
    ragEnabled: true,
    alertsEnabled: true,
    warnings: [],
  };
}

function sessionFixture(overrides: Partial<AuthSession> = {}): AuthSession {
  return {
    enabled: true,
    authenticated: false,
    permissions: ["read", "assist"],
    ...overrides,
  };
}

describe("ProfilePanel", () => {
  it("uses sanitized bearer token during authentication", async () => {
    const login = vi.fn().mockResolvedValue(
      sessionFixture({
        authenticated: true,
        user: { name: "alice", role: "operator" },
      }),
    );
    const refreshSession = vi.fn().mockResolvedValue(
      sessionFixture({
        authenticated: true,
        user: { name: "alice", role: "operator" },
      }),
    );
    const logout = vi.fn().mockResolvedValue(undefined);
    const setAuthToken = vi.fn();
    const onAuthMessage = vi.fn();

    render(
      <ProfilePanel
        runtime={runtimeFixture()}
        authSession={sessionFixture()}
        authLoading={false}
        authToken="Bearer    token-abc"
        setAuthToken={setAuthToken}
        authMessage={null}
        onAuthMessage={onAuthMessage}
        login={login}
        logout={logout}
        refreshSession={refreshSession}
        authLastRefreshAt={null}
        authLastLoginAt={null}
        authLastLogoutAt={null}
        authFailedLoginCount={0}
        currentCommand="kubectl get pods -A"
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Authenticate" }));

    await waitFor(() => {
      expect(login).toHaveBeenCalledWith("token-abc");
      expect(refreshSession).toHaveBeenCalledTimes(1);
      expect(setAuthToken).toHaveBeenCalledWith("");
      expect(onAuthMessage).toHaveBeenLastCalledWith("Session authenticated as alice.");
    });
  });

  it("disables authenticate action for bearer-only input", () => {
    render(
      <ProfilePanel
        runtime={runtimeFixture()}
        authSession={sessionFixture()}
        authLoading={false}
        authToken="Bearer"
        setAuthToken={vi.fn()}
        authMessage={null}
        onAuthMessage={vi.fn()}
        login={vi.fn().mockResolvedValue(sessionFixture())}
        logout={vi.fn().mockResolvedValue(undefined)}
        refreshSession={vi.fn().mockResolvedValue(sessionFixture())}
        authLastRefreshAt={null}
        authLastLoginAt={null}
        authLastLogoutAt={null}
        authFailedLoginCount={0}
        currentCommand="kubectl get pods -A"
      />,
    );

    expect(screen.getByRole("button", { name: "Authenticate" })).toBeDisabled();
    expect(screen.getByText("Bearer prefix will be stripped before authentication.")).toBeInTheDocument();
  });

  it("shows no-auth-required message when auth is disabled", async () => {
    const onAuthMessage = vi.fn();

    render(
      <ProfilePanel
        runtime={runtimeFixture()}
        authSession={sessionFixture({ enabled: false })}
        authLoading={false}
        authToken="token"
        setAuthToken={vi.fn()}
        authMessage={null}
        onAuthMessage={onAuthMessage}
        login={vi.fn().mockResolvedValue(sessionFixture())}
        logout={vi.fn().mockResolvedValue(undefined)}
        refreshSession={vi.fn().mockResolvedValue(sessionFixture())}
        authLastRefreshAt={null}
        authLastLoginAt={null}
        authLastLogoutAt={null}
        authFailedLoginCount={0}
        currentCommand="kubectl get pods -A"
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Authenticate" }));
    expect(onAuthMessage).toHaveBeenCalledWith("Auth is disabled in this environment. Sign-in is not required.");
  });
});
