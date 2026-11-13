const DEFAULT_BRANCH_UNRESOLVED_CODE = "DEFAULT_BRANCH_UNRESOLVED";

function extractWorkspaceRepoName(message: string): string | null {
	const namedRepo = message.match(/resolve workspace repo\s+"([^"]+)"/i);
	if (namedRepo?.[1]) return namedRepo[1];
	return null;
}

export function formatOrchestratorStartupError(message: string): string {
	if (!message.includes(DEFAULT_BRANCH_UNRESOLVED_CODE)) return message;
	const repoName = extractWorkspaceRepoName(message);
	const isChild = repoName !== null && repoName !== "__root__";
	const repoLabel = isChild ? `child repository "${repoName}"` : repoName ? "workspace root repository" : "project repository";
	const guidance = isChild
		? "Check this child's local base branch, then retry starting the orchestrator."
		: "Choose an available local base branch in Project Settings, then retry starting the orchestrator.";
	return `Project added, but orchestrator did not start. AO could not determine the default branch for the ${repoLabel}. ${guidance}\n\nDetails: ${message}`;
}
