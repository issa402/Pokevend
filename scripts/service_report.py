import asyncio
import os 
import sys 
import logging 
from typing import List, Optional, Dict, Any 
import requests

logging.basicConfig(
	level = logging.INFO,
	format = "%(asctime)s [%(levelname)s] %(message)s",
	datefmt = "%H:%M:%S"
)

logger = logging.getLogger("service_report")

def check_health(name, url, expected):
    try:
        r = requests.get(url, timeout= 5)
        if r.status_code == expected:
            logging.info(f"{name} is UP and RUNNING: {r.status_code}")
            return True
        else:
            logging.error(f"{name} is NOT UP and RUNNING: {r.status_code}")
            return False
    except Exception as e:
        logging.error(f"No connection: {e}")
    

check_health("Go API", "http://localhost:3001/health", 200)
check_health("Fast API", "http://localhost:8001/health", 200)


