// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";

afterEach(() => {
	vi.doUnmock("../../../backend/internal/buildpolicy/app-updates.json");
	vi.resetModules();
});

it.each([
	[true, true],
	[false, false],
	[undefined, false],
	[null, false],
	["true", false],
])("only an explicit boolean enables app updates (%s)", async (enabled, expected) => {
	vi.resetModules();
	vi.doMock("../../../backend/internal/buildpolicy/app-updates.json", () => ({ default: { enabled } }));
	const { APP_UPDATES_ENABLED } = await import("./app-update-policy");
	expect(APP_UPDATES_ENABLED).toBe(expected);
});
