#!/bin/bash

export CLIENT_ID=cf
export COMMAND="$(cat ~/tripe-c-run-test)"
export CONFIG_PATH=ci/config.yml
export PORT=8080
export REFRESH_TOKEN=$(cat ~/.cf/config.json | jq -r .RefreshToken)
export REPO_PATH=https://github.com/apoydence/triple-c
export VCAP_APPLICATION='{"cf_api":"https://api.coconut.cf-app.com","application_id":"1589500e-e676-4d73-a66c-59863da04522"}'

./triple-c
