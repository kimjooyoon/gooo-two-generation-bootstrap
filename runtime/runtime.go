package runtime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type Status string

const (
	StatusClosed  Status = "CLOSED"
	StatusUnknown Status = "UNKNOWN"
	StatusRefuted Status = "REFUTED"
)

type UnknownRecord struct {
	Stage         string `json:"stage"`
	Step          string `json:"step"`
	Reason        string `json:"reason"`
	UnknownClass  string `json:"unknown_class"`
	NextOperation string `json:"next_operation"`
	BlockedBy     string `json:"blocked_by"`
}

func (u UnknownRecord) Valid() bool {
	return u.Stage != "" && u.Step != "" && u.Reason != "" && u.UnknownClass != "" && u.NextOperation != "" && u.BlockedBy != ""
}

type ParserRule struct {
	ID           string   `json:"id"`
	Keyword      string   `json:"keyword"`
	Node         string   `json:"node"`
	Operands     int      `json:"operands"`
	OperandKinds []string `json:"operand_kinds"`
}

type LanguageSpec struct {
	Name            string            `json:"name"`
	SourceExtension string            `json:"source_extension"`
	ParserRules     []ParserRule      `json:"parser_rules"`
	Nodes           map[string]string `json:"nodes"`
	IR              IRSemantics       `json:"ir"`
	Emitter         EmitterSpec       `json:"emitter"`
}

type IRSemantics struct {
	Schema        string   `json:"schema"`
	Normalization []string `json:"normalization"`
	Fields        []string `json:"fields"`
}

type EmitterSpec struct {
	Schema   string `json:"schema"`
	Version  string `json:"version"`
	Output   string `json:"output"`
	Template string `json:"template"`
}

type GenerationPlan struct {
	Stage0         string   `json:"stage0"`
	Stage1         string   `json:"stage1"`
	SameInput      string   `json:"same_input"`
	Compare        []string `json:"compare"`
	OutputBoundary string   `json:"output_boundary"`
}

type CorpusCase struct {
	ID             string         `json:"id"`
	Source         string         `json:"source"`
	ExpectedStatus Status         `json:"expected_status"`
	ExpectedReason string         `json:"expected_reason"`
	Unknown        *UnknownRecord `json:"unknown,omitempty"`
}

type CorpusSpec struct {
	Selected []string     `json:"selected"`
	Cases    []CorpusCase `json:"cases"`
}

type ResolutionSpec struct {
	StatusPrecedence []Status      `json:"status_precedence"`
	UnknownFields    []string      `json:"unknown_fields"`
	UnknownTemplate  UnknownRecord `json:"unknown_template"`
	DivergenceStatus Status        `json:"divergence_status"`
}

type ImprovementSpec struct {
	SameScenarioSourceContractToolchain bool   `json:"same_scenario_source_contract_toolchain"`
	IntegerBeforeAfterRequired          bool   `json:"integer_before_after_required"`
	MissingEvidenceStatus               Status `json:"missing_evidence_status"`
}

type MeasurementSpec struct {
	Inventory          []string `json:"inventory"`
	Runtime            []string `json:"runtime"`
	TestCounts         []string `json:"test_counts"`
	GeneratedArtifacts []string `json:"generated_artifacts"`
}

type MetaContract struct {
	Schema         string          `json:"schema"`
	Authority      string          `json:"authority"`
	ContractID     string          `json:"contract_id"`
	Language       LanguageSpec    `json:"language"`
	GenerationPlan GenerationPlan  `json:"generation_plan"`
	Resolution     ResolutionSpec  `json:"resolution"`
	Improvement    ImprovementSpec `json:"improvement"`
	Measurements   MeasurementSpec `json:"measurements"`
	Corpus         CorpusSpec      `json:"corpus"`
}

type Binding struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

type Diagnostic struct {
	Line   int    `json:"line"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

type SemanticIR struct {
	Schema      string       `json:"schema"`
	Program     string       `json:"program"`
	Bindings    []Binding    `json:"bindings"`
	Emissions   []string     `json:"emissions"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type ParseOutcome struct {
	Status  Status         `json:"status"`
	Reason  string         `json:"reason"`
	Unknown *UnknownRecord `json:"unknown,omitempty"`
	IR      SemanticIR     `json:"ir"`
}

type GeneratedArtifact struct {
	Schema                string `json:"schema"`
	Kind                  string `json:"kind"`
	ContractDigest        string `json:"contract_digest"`
	SourceDigest          string `json:"source_digest"`
	IRDigest              string `json:"ir_digest"`
	EmitterDigest         string `json:"emitter_digest"`
	GeneratedSourceDigest string `json:"generated_source_digest"`
}

type StageResult struct {
	Schema                  string            `json:"schema"`
	Stage                   string            `json:"stage"`
	Status                  Status            `json:"status"`
	Reason                  string            `json:"reason"`
	Unknown                 *UnknownRecord    `json:"unknown,omitempty"`
	ContractDigest          string            `json:"contract_digest"`
	SourceDigest            string            `json:"source_digest"`
	IRDigest                string            `json:"ir_digest"`
	IR                      SemanticIR        `json:"ir"`
	GeneratedArtifactDigest string            `json:"generated_artifact_digest"`
	GeneratedArtifact       GeneratedArtifact `json:"generated_artifact"`
}

type Divergence struct {
	Status  Status         `json:"status"`
	Path    string         `json:"path"`
	Reason  string         `json:"reason"`
	Unknown *UnknownRecord `json:"unknown,omitempty"`
}

type FixedPointReport struct {
	Schema                 string         `json:"schema"`
	Status                 Status         `json:"status"`
	Reason                 string         `json:"reason"`
	Precedence             []Status       `json:"precedence"`
	Stage1IRDigest         string         `json:"stage1_ir_digest"`
	Stage2IRDigest         string         `json:"stage2_ir_digest"`
	Stage1ArtifactDigest   string         `json:"stage1_generated_artifact_digest"`
	Stage2ArtifactDigest   string         `json:"stage2_generated_artifact_digest"`
	SemanticIREqual        bool           `json:"semantic_ir_equal"`
	GeneratedArtifactEqual bool           `json:"generated_artifact_equal"`
	FirstDivergence        *Divergence    `json:"first_divergence,omitempty"`
	Unknown                *UnknownRecord `json:"unknown,omitempty"`
}

func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func DigestJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return Digest(data)
}

func LoadMeta(path string) (MetaContract, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return MetaContract{}, nil, err
	}
	meta, err := DecodeMeta(raw)
	return meta, raw, err
}

func DecodeMeta(raw []byte) (MetaContract, error) {
	var meta MetaContract
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&meta); err != nil {
		return MetaContract{}, fmt.Errorf("parse authoritative .gooo metacode: %w", err)
	}
	if err := meta.Validate(); err != nil {
		return MetaContract{}, err
	}
	return meta, nil
}

func (m MetaContract) Validate() error {
	if m.Schema != "gooo.self-hosting/v1" || m.Authority != "metacode" || m.ContractID == "" {
		return errors.New("self-hosting metacode must declare schema, contract_id, and metacode authority")
	}
	if m.Language.Name != "Gooo" || m.Language.SourceExtension != ".gooo" || len(m.Language.ParserRules) != 3 {
		return errors.New("language declaration must describe the three-rule Gooo slice")
	}
	seenKeywords := map[string]bool{}
	seenNodes := map[string]bool{}
	for _, rule := range m.Language.ParserRules {
		if rule.ID == "" || rule.Keyword == "" || rule.Node == "" || seenKeywords[rule.Keyword] || len(rule.OperandKinds) != rule.Operands {
			return fmt.Errorf("invalid parser rule %q", rule.ID)
		}
		seenKeywords[rule.Keyword] = true
		seenNodes[rule.Node] = true
	}
	for _, node := range []string{"program", "binding", "emission"} {
		if m.Language.Nodes[node] == "" || !seenNodes[m.Language.Nodes[node]] {
			return fmt.Errorf("missing parser node %q", node)
		}
	}
	if m.Language.IR.Schema != "gooo.semantic-ir/v1" || len(m.Language.IR.Fields) != 5 || len(m.Language.IR.Normalization) != 3 {
		return errors.New("semantic IR declaration is incomplete")
	}
	if m.Language.Emitter.Schema != "gooo.generated-artifact/v1" || m.Language.Emitter.Version == "" || m.Language.Emitter.Template == "" {
		return errors.New("emitter declaration is incomplete")
	}
	if m.GenerationPlan.Stage0 == "" || m.GenerationPlan.Stage1 == "" || m.GenerationPlan.SameInput != "exact_same_gooo_bytes" || len(m.GenerationPlan.Compare) != 2 || m.GenerationPlan.OutputBoundary != "caller_owned_temporary_only" {
		return errors.New("generation plan does not declare the two-generation boundary")
	}
	if !equalStatus(m.Resolution.StatusPrecedence, []Status{StatusRefuted, StatusUnknown, StatusClosed}) || !equalStrings(m.Resolution.UnknownFields, []string{"stage", "step", "reason", "unknown_class", "next_operation", "blocked_by"}) || !m.Resolution.UnknownTemplate.Valid() || m.Resolution.DivergenceStatus != StatusRefuted {
		return errors.New("resolution policy is incomplete")
	}
	if !m.Improvement.SameScenarioSourceContractToolchain || !m.Improvement.IntegerBeforeAfterRequired || m.Improvement.MissingEvidenceStatus != StatusUnknown {
		return errors.New("improvement evidence policy is incomplete")
	}
	if !equalStrings(m.Measurements.Inventory, []string{"go_files", "gooo_files", "go_physical_lines", "gooo_physical_lines", "subdirectories", "regular_files"}) || !equalStrings(m.Measurements.Runtime, []string{"compile_wall_ms", "build_wall_ms", "test_wall_ms", "conformance_wall_ms", "integration_wall_ms", "peak_rss_kib"}) || !equalStrings(m.Measurements.TestCounts, []string{"total", "selected", "executed", "reused", "failed", "unknown"}) || !equalStrings(m.Measurements.GeneratedArtifacts, []string{"count", "bytes"}) {
		return errors.New("measurement declaration is incomplete")
	}
	if len(m.Corpus.Cases) != 6 || len(m.Corpus.Selected) != 6 {
		return errors.New("corpus denominator must contain exactly six selected cases")
	}
	caseIDs := map[string]bool{}
	for _, item := range m.Corpus.Cases {
		if item.ID == "" || item.Source == "" || caseIDs[item.ID] {
			return fmt.Errorf("invalid or duplicate corpus case %q", item.ID)
		}
		caseIDs[item.ID] = true
		if item.ExpectedStatus == StatusUnknown {
			if item.Unknown == nil || !item.Unknown.Valid() {
				return fmt.Errorf("UNKNOWN corpus case %q lacks all six fields", item.ID)
			}
		}
	}
	for _, id := range m.Corpus.Selected {
		if !caseIDs[id] {
			return fmt.Errorf("selected corpus case %q is undeclared", id)
		}
	}
	return nil
}

func equalStatus(actual, expected []Status) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range actual {
		if actual[i] != expected[i] {
			return false
		}
	}
	return true
}

func equalStrings(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range actual {
		if actual[i] != expected[i] {
			return false
		}
	}
	return true
}

func Parse(meta MetaContract, source []byte) ParseOutcome {
	rules := map[string]ParserRule{}
	for _, rule := range meta.Language.ParserRules {
		rules[rule.Keyword] = rule
	}
	programNode := meta.Language.Nodes["program"]
	bindingNode := meta.Language.Nodes["binding"]
	emissionNode := meta.Language.Nodes["emission"]
	program := ""
	bindings := map[string]int64{}
	emissions := []string{}
	diagnostics := []Diagnostic{}
	var unknown *UnknownRecord
	status := StatusClosed
	reason := "exact parse and semantic resolution"
	setUnknown := func(line int, detail string) {
		if unknown != nil {
			return
		}
		u := meta.Resolution.UnknownTemplate
		u.Step = fmt.Sprintf("%s-line-%d", u.Step, line)
		u.Reason = detail
		unknown = &u
		if status == StatusClosed {
			status = StatusUnknown
			reason = detail
		}
		diagnostics = append(diagnostics, Diagnostic{Line: line, Code: "UNKNOWN", Detail: detail})
	}
	setRefuted := func(line int, code, detail string) {
		status = StatusRefuted
		reason = detail
		diagnostics = append(diagnostics, Diagnostic{Line: line, Code: code, Detail: detail})
	}
	lines := strings.Split(strings.ReplaceAll(string(source), "\r\n", "\n"), "\n")
	seenProgram := false
	seenEmission := false
	for lineNumber, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		rule, ok := rules[fields[0]]
		if !ok {
			setUnknown(lineNumber+1, "no declared parser rule matches token "+fields[0])
			continue
		}
		if len(fields)-1 != rule.Operands {
			setRefuted(lineNumber+1, "MALFORMED_"+strings.ToUpper(rule.ID), "declared rule "+rule.ID+" has the wrong operand count")
			continue
		}
		validOperands := true
		for index, kind := range rule.OperandKinds {
			operand := fields[index+1]
			if !validOperand(operand, kind) {
				validOperands = false
				setRefuted(lineNumber+1, "MALFORMED_"+strings.ToUpper(rule.ID), "operand "+strconv.Itoa(index+1)+" does not satisfy "+kind)
				break
			}
		}
		if !validOperands {
			continue
		}
		switch rule.Node {
		case programNode:
			if seenProgram {
				setRefuted(lineNumber+1, "DUPLICATE_PROGRAM", "program header is declared more than once")
				continue
			}
			seenProgram = true
			program = fields[1]
		case bindingNode:
			if _, exists := bindings[fields[1]]; exists {
				setRefuted(lineNumber+1, "DUPLICATE_BINDING", "binding "+fields[1]+" is declared more than once")
				continue
			}
			value, _ := strconv.ParseInt(fields[3], 10, 64)
			bindings[fields[1]] = value
		case emissionNode:
			seenEmission = true
			emissions = append(emissions, fields[1])
		}
	}
	if !seenProgram {
		setRefuted(0, "PROGRAM_REQUIRED", "a program header is required")
	}
	if !seenEmission {
		setRefuted(0, "EMIT_REQUIRED", "at least one emit statement is required")
	}
	for _, name := range emissions {
		if _, ok := bindings[name]; !ok {
			setUnknown(len(lines), "emitted binding "+name+" has no declaration")
		}
	}
	orderedBindings := make([]Binding, 0, len(bindings))
	for name, value := range bindings {
		orderedBindings = append(orderedBindings, Binding{Name: name, Value: value})
	}
	sort.Slice(orderedBindings, func(i, j int) bool { return orderedBindings[i].Name < orderedBindings[j].Name })
	if status == StatusRefuted {
		unknown = nil
	}
	return ParseOutcome{Status: status, Reason: reason, Unknown: unknown, IR: SemanticIR{Schema: meta.Language.IR.Schema, Program: program, Bindings: orderedBindings, Emissions: emissions, Diagnostics: diagnostics}}
}

func validOperand(value, kind string) bool {
	switch kind {
	case "identifier":
		return isIdentifier(value)
	case "equals":
		return value == "="
	case "integer":
		_, err := strconv.ParseInt(value, 10, 64)
		return err == nil
	default:
		return false
	}
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func IRBytes(ir SemanticIR) []byte {
	data, err := json.Marshal(ir)
	if err != nil {
		panic(err)
	}
	return data
}

func IRDigest(ir SemanticIR) string {
	return Digest(IRBytes(ir))
}

func RenderStage2Source(contractDigest, sourceDigest, irDigest string) []byte {
	return []byte(fmt.Sprintf("// Code generated by the Gooo two-generation emitter; DO NOT EDIT.\npackage main\n\nconst (\n\tGeneratedArtifactSchema = %q\n\tContractDigest = %q\n\tSourceDigest = %q\n\tSemanticIRDigest = %q\n)\n\nfunc main() {}\n", "gooo.generated-artifact/v1", contractDigest, sourceDigest, irDigest))
}

func BuildArtifact(meta MetaContract, metaRaw, source []byte, ir SemanticIR) (GeneratedArtifact, []byte, string) {
	contractDigest := Digest(metaRaw)
	sourceDigest := Digest(source)
	irDigest := IRDigest(ir)
	emitterDigest := DigestJSON(meta.Language.Emitter)
	sourceBytes := RenderStage2Source(contractDigest, sourceDigest, irDigest)
	artifact := GeneratedArtifact{Schema: meta.Language.Emitter.Schema, Kind: meta.Language.Emitter.Output, ContractDigest: contractDigest, SourceDigest: sourceDigest, IRDigest: irDigest, EmitterDigest: emitterDigest, GeneratedSourceDigest: Digest(sourceBytes)}
	return artifact, sourceBytes, DigestJSON(artifact)
}

func BuildStage1Source(metaRaw []byte) []byte {
	quoted := strconv.Quote(string(metaRaw))
	return []byte("// Code generated by trusted stage0 from authoritative .gooo metacode; DO NOT EDIT.\npackage main\n\nimport (\n\t\"flag\"\n\t\"fmt\"\n\n\tgoooruntime \"github.com/kimjooyoon/gooo-two-generation-bootstrap/runtime\"\n)\n\nconst embeddedMeta = " + quoted + "\n\nfunc main() {\n\tinput := flag.String(\"input\", \"\", \"same .gooo input used by stage0\")\n\toutputDir := flag.String(\"output-dir\", \"\", \"caller-owned stage2 output directory\")\n\trepoRoot := flag.String(\"repo-root\", \".\", \"target repository root\")\n\tflag.Parse()\n\tif err := goooruntime.ExecuteEmbedded([]byte(embeddedMeta), *input, *outputDir, *repoRoot, \"stage2\"); err != nil {\n\t\tfmt.Println(err)\n\t\tpanic(err)\n\t}\n}\n")
}

func ExecuteStage0(metaPath, inputPath, outputDir, repoRoot string) error {
	meta, metaRaw, err := LoadMeta(metaPath)
	if err != nil {
		return err
	}
	return execute(meta, metaRaw, inputPath, outputDir, repoRoot, "stage1", true)
}

func ExecuteEmbedded(metaRaw []byte, inputPath, outputDir, repoRoot, stage string) error {
	meta, err := DecodeMeta(metaRaw)
	if err != nil {
		return err
	}
	return execute(meta, metaRaw, inputPath, outputDir, repoRoot, stage, false)
}

func execute(meta MetaContract, metaRaw []byte, inputPath, outputDir, repoRoot, stage string, emitStage1 bool) error {
	if inputPath == "" || outputDir == "" {
		return errors.New("input and output-dir are required")
	}
	if err := EnsureCallerOwned(outputDir, repoRoot); err != nil {
		return err
	}
	source, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}
	outcome := Parse(meta, source)
	if outcome.Status != StatusClosed {
		return fmt.Errorf("%s input is %s: %s", stage, outcome.Status, outcome.Reason)
	}
	artifact, stage2Source, artifactDigest := BuildArtifact(meta, metaRaw, source, outcome.IR)
	result := StageResult{Schema: "gooo.stage-result/v1", Stage: stage, Status: outcome.Status, Reason: outcome.Reason, ContractDigest: Digest(metaRaw), SourceDigest: Digest(source), IRDigest: IRDigest(outcome.IR), IR: outcome.IR, GeneratedArtifactDigest: artifactDigest, GeneratedArtifact: artifact}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	if emitStage1 {
		if err := os.WriteFile(filepath.Join(outputDir, "stage1.go"), BuildStage1Source(metaRaw), 0o644); err != nil {
			return err
		}
	} else {
		if err := os.WriteFile(filepath.Join(outputDir, "stage2.go"), stage2Source, 0o644); err != nil {
			return err
		}
	}
	return WriteJSON(filepath.Join(outputDir, stage+"-result.json"), result)
}

func EnsureCallerOwned(outputDir, repoRoot string) error {
	out, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, out)
	if err != nil {
		return err
	}
	if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		return errors.New("generated output must be outside the target repository")
	}
	return nil
}

func LoadStageResult(path string) (StageResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StageResult{}, err
	}
	var result StageResult
	if err := json.Unmarshal(data, &result); err != nil {
		return StageResult{}, err
	}
	if result.Schema != "gooo.stage-result/v1" || result.Status == "" {
		return StageResult{}, errors.New("invalid stage result")
	}
	return result, nil
}

func Compare(stage1Dir, stage2Dir string, meta MetaContract) FixedPointReport {
	first, err1 := LoadStageResult(filepath.Join(stage1Dir, "stage1-result.json"))
	second, err2 := LoadStageResult(filepath.Join(stage2Dir, "stage2-result.json"))
	report := FixedPointReport{Schema: "gooo.fixed-point/v1", Precedence: []Status{StatusRefuted, StatusUnknown, StatusClosed}}
	if err1 != nil || err2 != nil {
		u := meta.Resolution.UnknownTemplate
		u.Step = "read-stage-results"
		u.Reason = "stage result evidence is missing or malformed"
		u.UnknownClass = "missing-evidence"
		u.NextOperation = "restore-stage-results"
		u.BlockedBy = "stage-result"
		report.Status = StatusUnknown
		report.Reason = u.Reason
		report.Unknown = &u
		return report
	}
	report.Stage1IRDigest = first.IRDigest
	report.Stage2IRDigest = second.IRDigest
	report.Stage1ArtifactDigest = first.GeneratedArtifactDigest
	report.Stage2ArtifactDigest = second.GeneratedArtifactDigest
	report.SemanticIREqual = first.IRDigest == second.IRDigest
	report.GeneratedArtifactEqual = first.GeneratedArtifactDigest == second.GeneratedArtifactDigest
	if first.Status != StatusClosed || second.Status != StatusClosed {
		report.Status, report.Reason, report.Unknown = reduceObserved(first, second, meta)
		return report
	}
	if first.ContractDigest != second.ContractDigest {
		report.Status = StatusRefuted
		report.Reason = "contract digest diverged"
		report.FirstDivergence = &Divergence{Status: StatusRefuted, Path: "contract_digest", Reason: report.Reason}
		return report
	}
	if first.SourceDigest != second.SourceDigest {
		report.Status = StatusRefuted
		report.Reason = "same .gooo input digest was not preserved"
		report.FirstDivergence = &Divergence{Status: StatusRefuted, Path: "source_digest", Reason: report.Reason}
		return report
	}
	if !report.SemanticIREqual {
		report.Status = meta.Resolution.DivergenceStatus
		report.Reason = "normalized semantic IR diverged"
		report.FirstDivergence = &Divergence{Status: report.Status, Path: firstIRDifference(first.IR, second.IR), Reason: report.Reason}
		return report
	}
	if !report.GeneratedArtifactEqual {
		report.Status = meta.Resolution.DivergenceStatus
		report.Reason = "generated artifact digest diverged"
		report.FirstDivergence = &Divergence{Status: report.Status, Path: "generated_artifact.digest", Reason: report.Reason}
		return report
	}
	report.Status = StatusClosed
	report.Reason = "exact fixed point: normalized semantic IR and generated artifact digest match"
	return report
}

func reduceObserved(first, second StageResult, meta MetaContract) (Status, string, *UnknownRecord) {
	if first.Status == StatusRefuted || second.Status == StatusRefuted {
		return StatusRefuted, "stage input was refuted", nil
	}
	u := meta.Resolution.UnknownTemplate
	u.Step = "compare-stage-status"
	u.Reason = "stage status is unknown"
	u.UnknownClass = "stage-observation"
	u.NextOperation = "restore-stage-input-evidence"
	u.BlockedBy = "stage-status"
	return StatusUnknown, u.Reason, &u
}

func firstIRDifference(a, b SemanticIR) string {
	if a.Schema != b.Schema {
		return "semantic_ir.schema"
	}
	if a.Program != b.Program {
		return "semantic_ir.program"
	}
	if len(a.Bindings) != len(b.Bindings) {
		return "semantic_ir.bindings.length"
	}
	for index := range a.Bindings {
		if a.Bindings[index].Name != b.Bindings[index].Name {
			return fmt.Sprintf("semantic_ir.bindings[%d].name", index)
		}
		if a.Bindings[index].Value != b.Bindings[index].Value {
			return fmt.Sprintf("semantic_ir.bindings[%d].value", index)
		}
	}
	if len(a.Emissions) != len(b.Emissions) {
		return "semantic_ir.emissions.length"
	}
	for index := range a.Emissions {
		if a.Emissions[index] != b.Emissions[index] {
			return fmt.Sprintf("semantic_ir.emissions[%d]", index)
		}
	}
	if len(a.Diagnostics) != len(b.Diagnostics) {
		return "semantic_ir.diagnostics.length"
	}
	for index := range a.Diagnostics {
		if a.Diagnostics[index] != b.Diagnostics[index] {
			return fmt.Sprintf("semantic_ir.diagnostics[%d]", index)
		}
	}
	return "semantic_ir"
}

type CorpusResult struct {
	ID             string         `json:"id"`
	Source         string         `json:"source"`
	ExpectedStatus Status         `json:"expected_status"`
	ExpectedReason string         `json:"expected_reason"`
	Status         Status         `json:"status"`
	Reason         string         `json:"reason"`
	Unknown        *UnknownRecord `json:"unknown,omitempty"`
	Pass           bool           `json:"pass"`
}

type CorpusReport struct {
	Schema         string         `json:"schema"`
	Status         Status         `json:"status"`
	ContractDigest string         `json:"contract_digest"`
	Precedence     []Status       `json:"precedence"`
	Cases          []CorpusResult `json:"cases"`
	Summary        map[string]int `json:"summary"`
}

func RunCorpus(meta MetaContract, metaRaw []byte, repoRoot string) (CorpusReport, error) {
	report := CorpusReport{Schema: "gooo.corpus/v1", ContractDigest: Digest(metaRaw), Precedence: []Status{StatusRefuted, StatusUnknown, StatusClosed}, Summary: map[string]int{"total": 0, "selected": len(meta.Corpus.Selected), "executed": 0, "reused": 0, "failed": 0, "unknown": 0}}
	byID := map[string]CorpusCase{}
	for _, item := range meta.Corpus.Cases {
		byID[item.ID] = item
	}
	for _, id := range meta.Corpus.Selected {
		item := byID[id]
		source, err := os.ReadFile(filepath.Join(repoRoot, item.Source))
		if err != nil {
			return report, err
		}
		outcome := Parse(meta, source)
		pass := outcome.Status == item.ExpectedStatus && (item.ExpectedReason == "" || outcome.Reason == item.ExpectedReason)
		if outcome.Status == StatusUnknown && (outcome.Unknown == nil || !outcome.Unknown.Valid()) {
			pass = false
		}
		result := CorpusResult{ID: item.ID, Source: item.Source, ExpectedStatus: item.ExpectedStatus, ExpectedReason: item.ExpectedReason, Status: outcome.Status, Reason: outcome.Reason, Unknown: outcome.Unknown, Pass: pass}
		report.Cases = append(report.Cases, result)
		report.Summary["total"]++
		report.Summary["executed"]++
		if outcome.Status == StatusUnknown {
			report.Summary["unknown"]++
		}
		if !pass {
			report.Summary["failed"]++
		}
	}
	if report.Summary["failed"] == 0 {
		report.Status = StatusClosed
	} else {
		report.Status = StatusRefuted
	}
	return report, nil
}

func WriteJSON(path string, value any) error {
	if path == "" {
		return errors.New("output path is required")
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
