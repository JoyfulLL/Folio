#!/bin/sh

set -e;

conf_path_param="";

if [ "${FOLIO_CONF_PATH}" != "" ]; then
  conf_path_param="--conf-path=${FOLIO_CONF_PATH}";
fi

if [ $# -gt 0 ]; then
    exec "$@"
else
    exec /folio/folio server run ${conf_path_param};
fi
