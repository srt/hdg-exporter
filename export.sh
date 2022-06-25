#!/bin/bash

DB_FILE="../hdg-firmware/www/pages/hdg-webvisu/data/hdg_bavaria.sqlite-init"
sqlite3 "$DB_FILE" ".mode json" "select enum,key,desc,rsi_int from enum_option where enum <> ''" > enum_option.json
sqlite3 "$DB_FILE" ".mode json" "select id,enum,data_type,desc1,desc2,formatter from data" > data.json
