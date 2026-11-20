#!/usr/bin/env bash
# 合并官方前检查：PR/Issue 状态、修复表是否完整、官方新提交与本分支改动的重叠。
# 用法：先 git fetch origin，再在仓库根目录运行 fork/check-upstream.sh
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

UPSTREAM_REPO=Wei-Shaw/sub2api
UPSTREAM_REF=${UPSTREAM_REF:-origin/main}
TABLE=fork/PATCHES.md
BASE=$(git merge-base HEAD "$UPSTREAM_REF")

# 只取「修复表」一节的数据行：第 1 列提交标题，第 2 列来源。
rows() {
  awk '/^## 修复表/{on=1;next} /^## /{on=0} on && /^\| / && !/^\|---/ && !/^\| 提交标题/' "$TABLE"
}

echo "== 官方状态（基线 $(git log -1 --format='%h %cd' --date=short "$BASE")，官方最新 $(git log -1 --format='%h %cd' --date=short "$UPSTREAM_REF")）"
rows | while IFS='|' read -r _ title source _; do
  title=$(echo "$title" | xargs); source=$(echo "$source" | xargs)
  case "$source" in
    "PR #"*)
      n=${source#PR #}
      state=$(gh pr view "$n" -R "$UPSTREAM_REPO" --json state,mergedAt -q '.state + (if .mergedAt then " " + .mergedAt[0:10] else "" end)' 2>/dev/null || echo "查询失败")
      flag=""; [[ $state == MERGED* ]] && flag="  <-- 已合并，变基时丢掉/比对"
      printf '  %-12s %-22s %s%s\n' "$source" "$state" "$title" "$flag" ;;
    "Issue #"*)
      n=${source#Issue #}
      state=$(gh issue view "$n" -R "$UPSTREAM_REPO" --json state,closedAt -q '.state + (if .closedAt then " " + .closedAt[0:10] else "" end)' 2>/dev/null || echo "查询失败")
      flag=""; [[ $state == CLOSED* ]] && flag="  <-- 已关闭，看官方修法"
      printf '  %-12s %-22s %s%s\n' "$source" "$state" "$title" "$flag" ;;
    *)
      printf '  %-12s %-22s %s\n' "$source" "-" "$title" ;;
  esac
done

echo
echo "== 表里没有记录的提交"
missing=0
mapfile -t titles < <(rows | awk -F'|' '{gsub(/^ +| +$/,"",$2); print $2}')
while IFS= read -r line; do
  sha=${line%% *}; subject=${line#* }
  # 只改 fork/ 的提交（本表自身）不需要登记
  if [ -z "$(git diff-tree --no-commit-id --name-only -r "$sha" -- . ':(exclude)fork')" ]; then continue; fi
  found=0
  for t in "${titles[@]}"; do [[ -n $t && $subject == *"$t"* ]] && { found=1; break; }; done
  if [ $found = 0 ]; then echo "  $sha $subject"; missing=1; fi
done < <(git log --format='%h %s' "$BASE..HEAD")
[ $missing = 0 ] && echo "  （无）"

echo
echo "== 官方新提交中改过本分支同一批文件的（$(git rev-list --count "$BASE..$UPSTREAM_REF") 个新提交）"
mapfile -t files < <(git diff --name-only "$BASE" HEAD -- . ':(exclude)fork' ':(exclude)*_test.go' ':(exclude)*.spec.ts')
if [ ${#files[@]} -gt 0 ] && [ "$BASE" != "$(git rev-parse "$UPSTREAM_REF")" ]; then
  out=$(git log --format='  %h %ad %s' --date=short "$BASE..$UPSTREAM_REF" -- "${files[@]}")
  echo "${out:-  （无）}"
else
  echo "  （无）"
fi
