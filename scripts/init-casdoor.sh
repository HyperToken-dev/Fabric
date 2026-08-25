#!/usr/bin/env sh
set -eu

casdoor_endpoint=${CASDOOR_ENDPOINT:-http://casdoor:8000}
admin_username=${CASDOOR_ADMIN_USERNAME:-built-in/admin}
admin_password=${CASDOOR_ADMIN_PASSWORD:-123}

app_organization=${CASDOOR_APP_ORGANIZATION}
app_owner=${CASDOOR_APP_OWNER:-admin}
app_name=${CASDOOR_APP_NAME:-app-fabric}
app_display_name=${CASDOOR_APP_DISPLAY_NAME:-Fabric}
app_cert=${CASDOOR_APP_CERT:-cert-built-in}
app_client_id=${CASDOOR_APP_CLIENT_ID:-e289d5f6a25fee57ef2e}
app_client_secret=${CASDOOR_APP_CLIENT_SECRET:-6beb0ea6fe3cbbf6ec2d3853860ad8e68c013fd7}
app_redirect_uris=${CASDOOR_APP_REDIRECT_URIS:-http://localhost:9090/auth/callback}
app_enable_signup=${CASDOOR_APP_ENABLE_SIGNUP:-true}
app_enable_password=${CASDOOR_APP_ENABLE_PASSWORD:-true}
app_enable_signin_session=${CASDOOR_APP_ENABLE_SIGNIN_SESSION:-true}
app_disable_signin=${CASDOOR_APP_DISABLE_SIGNIN:-false}

auth_query="username=${admin_username}&password=${admin_password}"
app_id="${app_owner}/${app_name}"

json_escape() {
    printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

json_string_array() {
    old_ifs=$IFS
    IFS=','
    result=""
    for value in $1; do
        value=$(printf '%s' "$value" | sed 's/^ *//; s/ *$//')
        if [ -z "$value" ]; then
            continue
        fi
        escaped=$(json_escape "$value")
        if [ -n "$result" ]; then
            result="${result},"
        fi
        result="${result}\"${escaped}\""
    done
    IFS=$old_ifs
    printf '[%s]' "$result"
}

request() {
    method=$1
    url=$2
    body=${3:-}
    if [ -n "$body" ]; then
        curl -fsS -X "$method" "$url" -H 'Content-Type: application/json' --data "$body"
    else
        curl -fsS -X "$method" "$url"
    fi
}

ensure_ok() {
    response=$1
    action=$2
    if printf '%s' "$response" | grep -q '"status"[[:space:]]*:[[:space:]]*"ok"'; then
        return 0
    fi
    echo "Casdoor ${action} failed: ${response}" >&2
    return 1
}

wait_for_casdoor() {
    i=1
    while [ "$i" -le 60 ]; do
        if curl -fsS "${casdoor_endpoint}/.well-known/openid-configuration" >/dev/null 2>&1; then
            return 0
        fi
        sleep 2
        i=$((i + 1))
    done
    echo "Casdoor did not become ready at ${casdoor_endpoint}" >&2
    return 1
}

application_payload() {
    redirect_uris=$(json_string_array "$app_redirect_uris")
    cat <<EOF
{
  "owner": "${app_owner}",
  "name": "${app_name}",
  "displayName": "${app_display_name}",
  "organization": "${app_organization}",
  "cert": "${app_cert}",
  "clientId": "${app_client_id}",
  "clientSecret": "${app_client_secret}",
  "redirectUris": ${redirect_uris},
  "grantTypes": [
    "authorization_code",
    "refresh_token"
  ],
  "tokenFormat": "JWT",
  "tokenSigningMethod": "RS256",
  "tokenFields": [
    "id",
    "permissions",
    "name",
    "email",
    "avatar",
    "preferred_username"
  ],
  "expireInHours": 168,
  "refreshExpireInHours": 720,
  "enableSignUp": ${app_enable_signup},
  "enablePassword": ${app_enable_password},
  "enableSigninSession": ${app_enable_signin_session},
  "disableSignin": ${app_disable_signin},
  "formOffset": 2,
  "logo": "https://ibed.aysel.work/101dda6a-73b7-4277-994e-f5858ee64dba",
  "themeData": {
    "borderRadius": 6,
    "colorPrimary": "#5820b4",
    "isCompact": false,
    "isEnabled": false,
    "themeType": "default"
  },
  "providers": [
    {
      "name": "provider_password",
      "canSignIn": true,
      "canSignUp": true,
      "canUnlink": false,
      "prompted": false,
      "rule": "All",
      "provider": {
        "owner": "admin",
        "name": "provider_password",
        "type": "Password",
        "category": "Password",
        "displayName": "Password"
      }
    }
  ],
  "signinMethods": [
    {
      "displayName": "Password",
      "name": "Password",
      "rule": "All"
    }
  ]
}
EOF
}

wait_for_casdoor

payload=$(application_payload)
get_response=$(request GET "${casdoor_endpoint}/api/get-application?id=${app_id}&${auth_query}" || true)

if printf '%s' "$get_response" | grep -q '"name"[[:space:]]*:[[:space:]]*"'"$(json_escape "$app_name")"'"'; then
    # update_response=$(request POST "${casdoor_endpoint}/api/update-application?id=${app_id}&${auth_query}" "$payload")
    # ensure_ok "$update_response" "update application"
    # echo "Updated Casdoor application ${app_id}"
    echo "application ${app_name}(${app_id}) exist,no create operation needed."
else
    add_response=$(request POST "${casdoor_endpoint}/api/add-application?${auth_query}" "$payload")
    ensure_ok "$add_response" "add application"
    echo "Created Casdoor application ${app_id}"
fi
