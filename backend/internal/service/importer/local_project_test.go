package importer

import (
	"context"
	"os/exec"
	"reflect"
	"testing"
)

func TestLocalImportListsBranchesWithoutRequiringOrigin(t *testing.T) {
	repo := gitRepoWithOrigin(t)
	if out, err := exec.Command("git", "-C", repo, "remote", "remove", "origin").CombinedOutput(); err != nil {
		t.Fatalf("remove remote: %v (%s)", err, out)
	}
	if out, err := exec.Command("git", "-C", repo, "branch", "release/local").CombinedOutput(); err != nil {
		t.Fatalf("add local branch: %v (%s)", err, out)
	}
	result, err := New(Deps{Store: newFakeStore()}).Validate(context.Background(), ImportValidationInput{ImportKind: ImportKindProject, Path: repo})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsValid || result.NextStep != ImportNextStepContinue || result.Root.HasOrigin || len(result.Root.RequiredActions) != 0 {
		t.Fatalf("local import requires remote setup: %+v", result)
	}
	if !reflect.DeepEqual(result.Root.LocalBranches, []string{"main", "release/local"}) || result.Root.CurrentBranch != "main" {
		t.Fatalf("local branch choices = %+v", result.Root)
	}
}
