#!/bin/bash

cd /root/palisade || exit 1

/usr/bin/timeout --signal=TERM --kill-after=30s 3m \
    ./palisade process \
    >> /root/palisade/process_log 2>&1

/usr/bin/timeout --signal=TERM --kill-after=30s 3m \
    ./palisade process-sell \
    >> /root/palisade/process_sell_log 2>&1

/usr/bin/timeout --signal=TERM --kill-after=30s 3m \
    ./palisade paper-palisade-signals \
    >> /root/palisade/paper_palisade_signals_log 2>&1
