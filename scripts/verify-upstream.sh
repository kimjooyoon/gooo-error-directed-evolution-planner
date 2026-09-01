#!/usr/bin/env bash
set -Eeuo pipefail

root=${1:?repository root is required}
out=${2:?caller-owned output directory is required}
lock="$root/contracts/upstream-lock-v1.json"
mkdir -p "$out/assets"

records='[]'
while IFS=$'\t' read -r repository release tag tag_object target_commit asset asset_digest url; do
  [ -n "$repository" ] || continue
  release_json=$(gh api -H 'X-GitHub-Api-Version: 2026-03-10' "repos/$repository/releases/tags/$release")
  jq -e --arg tag "$tag" '.tag_name == $tag and .draft == false and .prerelease == false and .immutable == true' <<<"$release_json" >/dev/null
  ref_json=$(gh api "repos/$repository/git/ref/tags/$tag")
  test "$(jq -r '.object.type' <<<"$ref_json")" = tag
  test "$(jq -r '.object.sha' <<<"$ref_json")" = "$tag_object"
  tag_json=$(gh api "repos/$repository/git/tags/$tag_object")
  test "$(jq -r '.object.type' <<<"$tag_json")" = commit
  test "$(jq -r '.object.sha' <<<"$tag_json")" = "$target_commit"
  curl --fail --silent --show-error --location --output "$out/assets/$asset" "$url"
  observed="sha256:$(sha256sum "$out/assets/$asset" | awk '{print $1}')"
  test "$observed" = "$asset_digest"
  record=$(jq -n --arg repository "$repository" --arg release "$release" --arg tag "$tag" --arg tag_object "$tag_object" --arg target_commit "$target_commit" --arg asset "$asset" --arg digest "$observed" --arg url "$url" --argjson release_id "$(jq '.id' <<<"$release_json")" '{repository:$repository,release:$release,tag:$tag,tag_object:$tag_object,target_commit:$target_commit,asset:$asset,digest:$digest,url:$url,release_id:$release_id,immutable:true}')
  records=$(jq --argjson record "$record" '. + [$record]' <<<"$records")
done < <(jq -r '.inputs[] | [.repository,.release,("v" + (.release | ltrimstr("v"))),.tag_object,.target_commit,.asset,.asset_sha256,.url] | @tsv' "$lock")

jq -n --arg schema "gooo/error-directed-evolution-planner/upstream-verification/v1" --argjson inputs "$records" '{schema:$schema,authority:"GITHUB_RELEASE_API",inputs:$inputs,immutable_release_required:true,sibling_checkout_consumption:false}' > "$out/upstream-report.json"
