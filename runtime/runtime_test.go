package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func fixtureMeta(t *testing.T) (MetaContract, []byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", ".gooo", "two-generation.gooo"))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := DecodeMeta(data)
	if err != nil {
		t.Fatal(err)
	}
	return meta, data
}

func TestMetaContractAuthority(t *testing.T) {
	meta, _ := fixtureMeta(t)
	if meta.Authority != "metacode" || meta.GenerationPlan.SameInput != "exact_same_gooo_bytes" {
		t.Fatalf("unexpected authority or generation plan: %#v", meta)
	}
}

func TestParserBehaviorCorpus(t *testing.T) {
	meta, _ := fixtureMeta(t)
	want := map[string]Status{"normal-basic.gooo": StatusClosed, "normal-sorted-ir.gooo": StatusClosed, "unknown-token.gooo": StatusUnknown, "unknown-unbound.gooo": StatusUnknown, "refuted-duplicate.gooo": StatusRefuted, "refuted-malformed.gooo": StatusRefuted}
	for name, expected := range want {
		source, err := os.ReadFile(filepath.Join("..", "examples", "corpus", name))
		if err != nil {
			t.Fatal(err)
		}
		outcome := Parse(meta, source)
		if outcome.Status != expected {
			t.Fatalf("%s: got %s, want %s", name, outcome.Status, expected)
		}
		if outcome.Status == StatusUnknown && (outcome.Unknown == nil || !outcome.Unknown.Valid()) {
			t.Fatalf("%s: UNKNOWN lost required fields", name)
		}
	}
}

func TestArtifactIsDeterministic(t *testing.T) {
	meta, raw := fixtureMeta(t)
	source := []byte("program fixed\nlet answer = 42\nemit answer\n")
	outcome := Parse(meta, source)
	first, firstSource, firstDigest := BuildArtifact(meta, raw, source, outcome.IR)
	second, secondSource, secondDigest := BuildArtifact(meta, raw, source, outcome.IR)
	if firstDigest != secondDigest || Digest(firstSource) != Digest(secondSource) || first.GeneratedSourceDigest != second.GeneratedSourceDigest {
		t.Fatal("artifact generation is not deterministic")
	}
}

func TestMinimumDivergencePath(t *testing.T) {
	meta, raw := fixtureMeta(t)
	source := []byte("program fixed\nlet answer = 42\nemit answer\n")
	outcome := Parse(meta, source)
	artifact, _, artifactDigest := BuildArtifact(meta, raw, source, outcome.IR)
	left := StageResult{Schema: "gooo.stage-result/v1", Stage: "stage1", Status: StatusClosed, ContractDigest: Digest(raw), SourceDigest: Digest(source), IRDigest: IRDigest(outcome.IR), IR: outcome.IR, GeneratedArtifactDigest: artifactDigest, GeneratedArtifact: artifact}
	changed := outcome.IR
	changed.Bindings[0].Value = 43
	changedArtifact, _, changedDigest := BuildArtifact(meta, raw, source, changed)
	right := StageResult{Schema: "gooo.stage-result/v1", Stage: "stage2", Status: StatusClosed, ContractDigest: Digest(raw), SourceDigest: Digest(source), IRDigest: IRDigest(changed), IR: changed, GeneratedArtifactDigest: changedDigest, GeneratedArtifact: changedArtifact}
	if got := firstIRDifference(left.IR, right.IR); got != "semantic_ir.bindings[0].value" {
		t.Fatalf("got divergence path %q", got)
	}
}

func TestCallerOwnedBoundary(t *testing.T) {
	root := t.TempDir()
	if err := EnsureCallerOwned(filepath.Join(root, "outputs"), root); err == nil {
		t.Fatal("target-contained output was accepted")
	}
	if err := EnsureCallerOwned(filepath.Join(filepath.Dir(root), "outputs"), root); err != nil {
		t.Fatal(err)
	}
}
