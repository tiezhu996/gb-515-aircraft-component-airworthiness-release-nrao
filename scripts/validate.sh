#!/usr/bin/env sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"
set -a
if [ -f .env ]; then . ./.env; else . ./.env.example; fi
set +a

(command -v jq >/dev/null 2>&1) || { echo "jq is required for API validation" >&2; exit 1; }

(cd backend && go test ./... && go test -race ./... && go vet ./... && go build ./...)
(cd frontend && npm install --no-audit --no-fund && npm run typecheck && npm run build)
docker compose config --quiet
docker compose down -v --remove-orphans
docker compose up -d --build

cleanup() { docker compose down -v --remove-orphans; }
if [ "${KEEP_RUNNING:-0}" = "1" ]; then
  trap cleanup INT TERM
else
  trap cleanup EXIT INT TERM
fi

i=0
until curl -fsS "http://127.0.0.1:${BACKEND_PORT:-19515}/healthz" | jq -e '.data.status == "ok" and .data.database == "ready" and .data.redis == "ready"' >/dev/null; do
  i=$((i+1))
  [ "$i" -lt 60 ] || { docker compose logs; exit 1; }
  sleep 2
done
curl -fsS "http://127.0.0.1:${FRONTEND_PORT:-18515}/" >/dev/null

login_token() {
  curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT:-19515}/api/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$1\",\"password\":\"Admin123!\"}" | jq -er '.data.token'
}

post_transition() {
  # $1 token, $2 resource, $3 id, $4 target status, $5 expected version, $6 request id
  curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/$2/$3/transition" \
    -H "Authorization: Bearer $1" -H 'Content-Type: application/json' -H "X-Request-ID: $6" \
    -d "{\"status\":\"$4\",\"expectedVersion\":$5,\"reason\":\"$6\"}"
}

admin_token=$(login_token admin)
reviewer_token=$(login_token reviewer)
operator_token=$(login_token operator)
viewer_token=$(login_token viewer)

curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/session" -H "Authorization: Bearer $viewer_token" \
  | jq -e '.data.role == "viewer" and (.data.requestId | length > 0)' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/parts?page=1&pageSize=20" -H "Authorization: Bearer $viewer_token" \
  | jq -e '.data | length >= 3' >/dev/null

viewer_write_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/parts" \
  -H "Authorization: Bearer $viewer_token" -H 'Content-Type: application/json' -d '{}')
[ "$viewer_write_status" = "403" ]

now=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
suffix=$(date +%s)
part_code="PART-SMOKE-${suffix}"

# 放行前证据闸门依赖完整证据链：部件暂停、检查通过、证书有效且在有效期内。
part_payload=$(printf '{"code":"%s","name":"Validated evidence part","description":"Evidence gate validation part","facility":"Validation Hangar","owner":"Release Desk","category":"engine","riskLevel":"high","metricValue":100,"metricUnit":"percent","effectiveAt":"%s","evidence":"part held for release review","relatedCode":"REL-%s"}' "$part_code" "$now" "$part_code")
part=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/parts" \
  -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-part-create' \
  -d "$part_payload")
part_id=$(printf '%s' "$part" | jq -er '.data.id')
part=$(post_transition "$operator_token" parts "$part_id" inspection 1 gb515-part-inspection)
part=$(post_transition "$operator_token" parts "$part_id" hold 2 gb515-part-hold)
printf '%s' "$part" | jq -e '.data.status == "hold"' >/dev/null

inspection_payload=$(printf '{"code":"INSP-SMOKE-%s","name":"Validated release inspection","description":"Evidence gate validation inspection","facility":"Validation Hangar","owner":"Inspection Desk","category":"engine","riskLevel":"medium","metricValue":100,"metricUnit":"percent","effectiveAt":"%s","evidence":"inspection steps complete","relatedCode":"%s"}' "$suffix" "$now" "$part_code")
inspection=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/inspections" \
  -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-insp-create' \
  -d "$inspection_payload")
inspection_id=$(printf '%s' "$inspection" | jq -er '.data.id')
inspection=$(post_transition "$operator_token" inspections "$inspection_id" running 1 gb515-insp-running)
inspection=$(post_transition "$operator_token" inspections "$inspection_id" passed 2 gb515-insp-passed)
printf '%s' "$inspection" | jq -e '.data.status == "passed"' >/dev/null

certificate_payload=$(printf '{"code":"CERT-SMOKE-%s","name":"Validated airworthiness certificate","description":"Immutable certificate validation","facility":"Validation Hangar","owner":"Certificate Desk","category":"engine","riskLevel":"medium","metricValue":100,"metricUnit":"percent","effectiveAt":"%s","evidence":"inspection report IR-SMOKE","relatedCode":"%s"}' "$suffix" "$now" "$part_code")
certificate=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/certificates" \
  -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-cert-create' \
  -d "$certificate_payload")
certificate_id=$(printf '%s' "$certificate" | jq -er '.data.id')
certificate_version=$(printf '%s' "$certificate" | jq -er '.data.version')

operator_certificate_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/certificates/${certificate_id}/transition" \
  -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-cert-operator-denied' \
  -d "{\"status\":\"valid\",\"expectedVersion\":${certificate_version},\"reason\":\"operator must not publish\"}")
[ "$operator_certificate_status" = "403" ]

certificate_valid=$(post_transition "$reviewer_token" certificates "$certificate_id" valid "$certificate_version" gb515-cert-publish)
printf '%s' "$certificate_valid" | jq -e '.data.status == "valid" and .data.version == 2 and .data.preparedBy == "operator" and .data.verifiedBy == "reviewer" and (.data.revisions | length) == 2 and .data.revisions[1].requestId == "gb515-cert-publish"' >/dev/null

# 缺少证据时提交复核必须被拒绝，响应列出全部阻断项且原状态保留。
missing_payload=$(printf '{"code":"AUTH-SMOKE-MISSING-%s","name":"Gate blocked release","description":"Evidence gate negative validation","facility":"Validation Hangar","owner":"Release Desk","category":"engine","riskLevel":"high","metricValue":100,"metricUnit":"percent","effectiveAt":"%s","evidence":"no evidence chain attached","relatedCode":"PART-MISSING-%s"}' "$suffix" "$now" "$suffix")
missing=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/authorizations" \
  -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-gate-missing-create' \
  -d "$missing_payload")
missing_id=$(printf '%s' "$missing" | jq -er '.data.id')
gate_response=$(mktemp)
gate_status=$(curl -sS -o "$gate_response" -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/authorizations/${missing_id}/transition" \
  -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-gate-blocked' \
  -d '{"status":"review","expectedVersion":1,"reason":"submit without any evidence"}')
[ "$gate_status" = "422" ]
jq -e '.error == "evidence_gate_blocked" and (.meta.blockers | length) == 3' "$gate_response" >/dev/null
rm -f "$gate_response"
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/authorizations/${missing_id}" -H "Authorization: Bearer $viewer_token" \
  | jq -e '.data.status == "draft" and .data.version == 1' >/dev/null

authorization_payload=$(printf '{"code":"AUTH-SMOKE-%s","name":"Validated component release","description":"Dual-control Compose validation","facility":"Validation Hangar","owner":"Release Desk","category":"engine","riskLevel":"high","metricValue":100,"metricUnit":"percent","effectiveAt":"%s","evidence":"inspection IR-SMOKE and certificate CERT-SMOKE","relatedCode":"%s"}' "$suffix" "$now" "$part_code")
authorization=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/authorizations" \
  -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-auth-create' \
  -d "$authorization_payload")
authorization_id=$(printf '%s' "$authorization" | jq -er '.data.id')
authorization_version=$(printf '%s' "$authorization" | jq -er '.data.version')
printf '%s' "$authorization" | jq -e '.data.status == "draft" and .data.version == 1 and .data.revisions[0].actor == "operator" and .data.revisions[0].requestId == "gb515-auth-create"' >/dev/null

authorization_review=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/authorizations/${authorization_id}/transition" \
  -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-auth-review' \
  -d "{\"status\":\"review\",\"expectedVersion\":${authorization_version},\"reason\":\"inspection and certificate evidence complete\"}")
authorization_review_version=$(printf '%s' "$authorization_review" | jq -er '.data.version')
printf '%s' "$authorization_review" | jq -e '.data.status == "review" and .data.submittedBy == "operator" and (.data.revisions | length) == 2' >/dev/null

operator_approval_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/authorizations/${authorization_id}/transition" \
  -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-auth-operator-denied' \
  -d "{\"status\":\"approved\",\"expectedVersion\":${authorization_review_version},\"reason\":\"operator must not self approve\"}")
[ "$operator_approval_status" = "403" ]

authorization_approved=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/authorizations/${authorization_id}/transition" \
  -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-auth-approve' \
  -d "{\"status\":\"approved\",\"expectedVersion\":${authorization_review_version},\"reason\":\"independent airworthiness release review passed\"}")
printf '%s' "$authorization_approved" | jq -e '.data.status == "approved" and .data.version == 3 and .data.submittedBy == "operator" and .data.reviewedBy == "reviewer" and (.data.revisions | length) == 3 and .data.revisions[2].requestId == "gb515-auth-approve"' >/dev/null

# 等待复核期间证据变化必须阻断批准；发布最新有效证书后方可放行，重复批准不得二次生效。
pending_payload=$(printf '{"code":"AUTH-SMOKE-PENDING-%s","name":"Evidence revalidation release","description":"Approval re-reads latest evidence","facility":"Validation Hangar","owner":"Release Desk","category":"engine","riskLevel":"high","metricValue":100,"metricUnit":"percent","effectiveAt":"%s","evidence":"evidence chain under review","relatedCode":"%s"}' "$suffix" "$now" "$part_code")
pending=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/authorizations" \
  -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-pending-create' \
  -d "$pending_payload")
pending_id=$(printf '%s' "$pending" | jq -er '.data.id')
pending_review=$(post_transition "$operator_token" authorizations "$pending_id" review 1 gb515-pending-review)
printf '%s' "$pending_review" | jq -e '.data.status == "review" and .data.version == 2' >/dev/null

certificate_expired=$(post_transition "$reviewer_token" certificates "$certificate_id" expired 2 gb515-cert-expire)
printf '%s' "$certificate_expired" | jq -e '.data.status == "expired"' >/dev/null

stale_response=$(mktemp)
stale_status=$(curl -sS -o "$stale_response" -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/authorizations/${pending_id}/transition" \
  -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-pending-stale-denied' \
  -d '{"status":"approved","expectedVersion":2,"reason":"approve with stale evidence"}')
[ "$stale_status" = "422" ]
jq -e '.error == "evidence_gate_blocked" and (.meta.blockers | length) == 1' "$stale_response" >/dev/null
rm -f "$stale_response"
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/authorizations/${pending_id}" -H "Authorization: Bearer $viewer_token" \
  | jq -e '.data.status == "review" and .data.version == 2' >/dev/null

certificate2_payload=$(printf '{"code":"CERT-SMOKE-2-%s","name":"Renewed airworthiness certificate","description":"Latest certificate restores evidence","facility":"Validation Hangar","owner":"Certificate Desk","category":"engine","riskLevel":"medium","metricValue":100,"metricUnit":"percent","effectiveAt":"%s","evidence":"renewed inspection report IR-SMOKE-2","relatedCode":"%s"}' "$suffix" "$now" "$part_code")
certificate2=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/certificates" \
  -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-cert2-create' \
  -d "$certificate2_payload")
certificate2_id=$(printf '%s' "$certificate2" | jq -er '.data.id')
certificate2_valid=$(post_transition "$reviewer_token" certificates "$certificate2_id" valid 1 gb515-cert2-publish)
printf '%s' "$certificate2_valid" | jq -e '.data.status == "valid"' >/dev/null

pending_approved=$(post_transition "$reviewer_token" authorizations "$pending_id" approved 2 gb515-pending-approve)
printf '%s' "$pending_approved" | jq -e '.data.status == "approved" and .data.version == 3 and .data.reviewedBy == "reviewer"' >/dev/null

repeat_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/authorizations/${pending_id}/transition" \
  -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -H 'X-Request-ID: gb515-pending-duplicate' \
  -d '{"status":"approved","expectedVersion":3,"reason":"duplicate approval must fail"}')
[ "$repeat_status" = "422" ]

curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/audits/ReleaseAuthorization/${authorization_id}?limit=10" \
  -H "Authorization: Bearer $admin_token" \
  | jq -e '[.data[].requestId] | index("gb515-auth-create") != null and index("gb515-auth-review") != null and index("gb515-auth-approve") != null' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/audit-summary?windowHours=24" -H "Authorization: Bearer $admin_token" \
  | jq -e '.data.total >= 5 and .data.transitions >= 3 and .data.uniqueActors >= 2' >/dev/null

docker compose ps
[ "${KEEP_RUNNING:-0}" = "1" ] && echo "KEEP_RUNNING=1: containers left running for built-in Browser validation"
