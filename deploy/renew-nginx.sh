#!/usr/bin/env bash
set -euo pipefail
case " ${RENEWED_DOMAINS:-} " in
    *" chat.linblog.site "*)
        nginx -t
        systemctl reload nginx
        ;;
esac
