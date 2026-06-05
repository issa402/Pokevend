#!/bin/bash

set -euo pipefail

VAR=$(lsblk | wc -l)
if [ "$VAR" -gt 38 ] ; then
	echo "More than normal devices detected"
else
	echo "ALL GOOD"
fi


