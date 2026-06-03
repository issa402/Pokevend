
#!/bin/bash 
clear
echo "
PLEASE SELECT:

1. Display System Information
2. Display Disk Space
3. Display Home Space Utilization
0. Quit
"
read -r -p "Enter selecttion [0-3] > "
case "$REPLY" in 
	0) 	echo "Program terminated."
		exit
		;;
	1)	echo "Hostname: $HOSTNAME"
		uptime
		;;
	2) 	echo "DISK SPACE"
		df -h
	3)	if [[ "$(id -u)" -eq 0 ]]; then 
			echo "Home Space Utilization (All Users)"
			du -sh /home/*
		else
			echo "Home Space Utilization ($USER)"
			du -sh "$HOME"
		fi
		;;
	*)	echo "Invalid entry" >&2
		exit 1
		;;
esac
