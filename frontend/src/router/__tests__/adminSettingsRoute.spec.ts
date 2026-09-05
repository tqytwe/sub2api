import { describe, expect, it, vi } from "vitest";

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: false,
  isAdmin: false,
  isSimpleMode: false,
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: () => authStore,
}));

vi.mock("@/stores/app", () => ({
  useAppStore: () => ({
    siteName: "Sub2API",
    backendModeEnabled: false,
    cachedPublicSettings: null,
  }),
}));

vi.mock("@/stores/adminSettings", () => ({
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}));

vi.mock("@/composables/useNavigationLoading", () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
    isLoading: { value: false },
  }),
}));

vi.mock("@/composables/useRoutePrefetch", () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}));

vi.mock("@/api", () => ({
  authAPI: { getCurrentUser: vi.fn(), logout: vi.fn() },
  isTotp2FARequired: () => false,
}));

vi.mock("@/api/admin/system", () => ({ checkUpdates: vi.fn() }));
vi.mock("@/api/auth", () => ({ getPublicSettings: vi.fn() }));

describe("admin settings route contract", () => {
  it("keeps system settings administrator-only with its lazy locale scope", async () => {
    const { default: router } = await import("@/router");
    const route = router.getRoutes().find((record) => record.name === "AdminSettings");

    expect(route?.path).toBe("/admin/settings");
    expect(route?.meta.requiresAuth).toBe(true);
    expect(route?.meta.requiresAdmin).toBe(true);
    expect(route?.meta.localeScopes).toEqual(["admin-settings"]);
  });
});
