#!/usr/bin/env bash
set -euo pipefail

root=${1:?repository root is required}
work=${2:?caller-owned work directory is required}
output=${3:?evidence output is required}
compile_time=${4:?compile timing file is required}
build_time=${5:?build timing file is required}
test_time=${6:?test timing file is required}
conformance_time=${7:?conformance timing file is required}
integration_time=${8:?integration timing file is required}
test_json=${9:?go test JSON is required}

read_metric() {
	local path=$1
	if [ -f "$path" ]; then
		awk '$1 ~ /^[0-9]+$/ && $2 ~ /^[0-9]+$/ {print $1 " " $2; found=1} END {if (!found) print "0 0"}' "$path"
	else
		printf '0 0\n'
	fi
}

read -r compile_ms compile_rss < <(read_metric "$compile_time")
read -r build_ms build_rss < <(read_metric "$build_time")
read -r test_ms test_rss < <(read_metric "$test_time")
read -r conformance_ms conformance_rss < <(read_metric "$conformance_time")
read -r integration_ms integration_rss < <(read_metric "$integration_time")

go_files=0
gooo_files=0
go_lines=0
gooo_lines=0
regular_files=0
subdirectories=0
while IFS= read -r -d '' path; do
	relative=${path#"$root"/}
	if [ "$relative" = "README.md" ] || [[ "$relative" == .git/* ]]; then
		continue
	fi
	if [ -d "$path" ]; then
		subdirectories=$((subdirectories + 1))
		continue
	fi
	if [ -f "$path" ]; then
		regular_files=$((regular_files + 1))
		case "$path" in
			*.go)
				go_files=$((go_files + 1))
				lines=$(wc -l < "$path" | tr -d ' ')
				go_lines=$((go_lines + lines))
				;;
			*.gooo)
				gooo_files=$((gooo_files + 1))
				lines=$(wc -l < "$path" | tr -d ' ')
				gooo_lines=$((gooo_lines + lines))
				;;
		esac
	fi
done < <(find "$root" -mindepth 1 -print0)

artifact_count=0
artifact_bytes=0
if [ -d "$work/artifacts" ]; then
	artifact_count=$(find "$work/artifacts" -type f | wc -l | tr -d ' ')
	if [ "$artifact_count" -gt 0 ]; then
		artifact_bytes=$(find "$work/artifacts" -type f -print0 | xargs -0 stat -c '%s' | awk '{sum += $1} END {print sum + 0}')
	fi
fi

test_total=$(jq -s '[.[] | select(.Action == "run" and (.Test // "") != "")] | length' "$test_json")
test_failed=$(jq -s '[.[] | select(.Action == "fail" and (.Test // "") != "")] | length' "$test_json")
test_reused=$(jq -s '[.[] | select(.Action == "output" and (.Output // "" | contains("(cached)")))] | length' "$test_json")
test_skipped=$(jq -s '[.[] | select(.Action == "skip" and (.Test // "") != "")] | length' "$test_json")
selected=$(jq -r '.summary.selected // 0' "$work/artifacts/conformance.json" 2>/dev/null || printf '0')
conformance_failed=$(jq -r '.summary.failed // 0' "$work/artifacts/conformance.json" 2>/dev/null || printf '0')
conformance_unknown=$(jq -r '.summary.unknown // 0' "$work/artifacts/conformance.json" 2>/dev/null || printf '0')
fixed_status=$(jq -r '.status // "UNKNOWN"' "$work/artifacts/fixed-point.json" 2>/dev/null || printf 'UNKNOWN')
corpus_status=$(jq -r '.status // "UNKNOWN"' "$work/artifacts/conformance.json" 2>/dev/null || printf 'UNKNOWN')

peak_rss=$compile_rss
for rss in "$build_rss" "$test_rss" "$conformance_rss" "$integration_rss"; do
	if [ "$rss" -gt "$peak_rss" ]; then peak_rss=$rss; fi
done

meta_digest=$(sha256sum "$root/.gooo/two-generation.gooo" | awk '{print "sha256:" $1}')
fixed_digest=$(if [ -f "$work/artifacts/fixed-point.json" ]; then sha256sum "$work/artifacts/fixed-point.json" | awk '{print "sha256:" $1}'; else printf 'sha256:missing'; fi)
stage1_artifact=$(jq -r '.stage1_generated_artifact_digest // "UNKNOWN"' "$work/artifacts/fixed-point.json" 2>/dev/null || printf 'UNKNOWN')
stage2_artifact=$(jq -r '.stage2_generated_artifact_digest // "UNKNOWN"' "$work/artifacts/fixed-point.json" 2>/dev/null || printf 'UNKNOWN')

jq -n \
	--arg schema 'gooo.selfhost-evidence/v1' \
	--arg authority 'gooo/two-generation.gooo' \
	--arg meta_digest "$meta_digest" \
	--arg fixed_digest "$fixed_digest" \
	--arg stage1_artifact "$stage1_artifact" \
	--arg stage2_artifact "$stage2_artifact" \
	--arg fixed_status "$fixed_status" \
	--arg corpus_status "$corpus_status" \
	--argjson go_files "$go_files" \
	--argjson gooo_files "$gooo_files" \
	--argjson go_lines "$go_lines" \
	--argjson gooo_lines "$gooo_lines" \
	--argjson subdirectories "$subdirectories" \
	--argjson regular_files "$regular_files" \
	--argjson compile_ms "$compile_ms" \
	--argjson build_ms "$build_ms" \
	--argjson test_ms "$test_ms" \
	--argjson conformance_ms "$conformance_ms" \
	--argjson integration_ms "$integration_ms" \
	--argjson peak_rss "$peak_rss" \
	--argjson test_total "$test_total" \
	--argjson selected "$selected" \
	--argjson test_failed "$test_failed" \
	--argjson test_reused "$test_reused" \
	--argjson test_skipped "$test_skipped" \
	--argjson conformance_failed "$conformance_failed" \
	--argjson conformance_unknown "$conformance_unknown" \
	--argjson artifact_count "$artifact_count" \
	--argjson artifact_bytes "$artifact_bytes" \
	'{schema:$schema,authority:$authority,contract_digest:$meta_digest,fixed_point_status:$fixed_status,corpus_status:$corpus_status,inventory:{go_files:$go_files,gooo_files:$gooo_files,go_physical_lines:$go_lines,gooo_physical_lines:$gooo_lines,subdirectories:$subdirectories,regular_files:$regular_files,root_readme_excluded:true},runtime:{compile_wall_ms:$compile_ms,build_wall_ms:$build_ms,test_wall_ms:$test_ms,conformance_wall_ms:$conformance_ms,integration_wall_ms:$integration_ms,peak_rss_kib:$peak_rss},tests:{total:$test_total,selected:$selected,executed:($test_total-$test_failed),reused:$test_reused,failed:($test_failed+$conformance_failed),unknown:($test_skipped+$conformance_unknown)},generated_artifacts:{count:$artifact_count,bytes:$artifact_bytes},digests:{fixed_point_report:$fixed_digest,stage1_generated_artifact:$stage1_artifact,stage2_generated_artifact:$stage2_artifact},authority_boundary:{local_test_executions:0,verification_authority:"github-actions",input_repository_modified:false,output_directory:"caller-owned-temporary"}}' \
	> "$output"
