#!/bin/bash

HOW_MANY=1000

for i in $(bzcat top-1m.csv.bz2 | head -n $HOW_MANY | awk -F, '{print $2}'); do curl \
 -d"url=$1" http://localhost:9999/shorten/ ; done
