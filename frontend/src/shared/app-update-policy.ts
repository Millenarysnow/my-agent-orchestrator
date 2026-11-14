import policy from "../../../backend/internal/buildpolicy/app-updates.json";

// Shared with the Go bootstrapper and baked into the app; user preferences and
// runtime environment variables cannot enable updates in a disabled fork build.
export const APP_UPDATES_ENABLED = policy.enabled === true;
export const APP_UPDATES_DISABLED_MESSAGE = "Application updates are disabled in this build.";
