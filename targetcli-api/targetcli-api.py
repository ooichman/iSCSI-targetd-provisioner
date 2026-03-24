#!/usr/bin/python3
import configparser
import os
import sys
import subprocess
import argparse
from flask import Flask, request, jsonify

# --- ARGUMENT PARSING ---
parser = argparse.ArgumentParser(description="TargetCLI JSON-RPC API for OpenShift Provisioning")
parser.add_argument("-f", "--config", 
                    default="/etc/targetcli-api.conf",
                    help="Path to the configuration file (default: /etc/targetcli-api.conf)")
args = parser.parse_args()

CONFIG_PATH = args.config
config = configparser.ConfigParser()

# Default values mapping
defaults = {
    'VG_NAME': 'vg-targetd',
    'TARGET_IQN': 'iqn.2026-03.com.example:target01',
    'TPG_TAG': '1',
    'API_HOST': '0.0.0.0',
    'API_PORT': '18700'
}

def get_config_or_default(section, key):
    if config.has_section(section) and config.has_option(section, key):
        return config.get(section, key)
    else:
        default_val = defaults.get(key)
        print(f"NOTE: Variable '{key}' not found in {CONFIG_PATH}. Using default: {default_val}")
        return default_val

# Try to read the file provided by -f
if os.path.exists(CONFIG_PATH):
    config.read(CONFIG_PATH)
else:
    print(f"WARNING: Configuration file {CONFIG_PATH} not found. Proceeding with system defaults.")

# Assign variables
VG_NAME    = get_config_or_default('STORAGE', 'VG_NAME')
TARGET_IQN = get_config_or_default('STORAGE', 'TARGET_IQN')
TPG_TAG    = int(get_config_or_default('STORAGE', 'TPG_TAG'))
API_HOST   = get_config_or_default('NETWORK', 'API_HOST')
API_PORT   = int(get_config_or_default('NETWORK', 'API_PORT'))

def verify_environment():
    """Verify that the predefined LVM Volume Group exists on the RHEL host."""
    check_vg = subprocess.run(['vgs', VG_NAME], capture_output=True, text=True)
    if check_vg.returncode != 0:
        print(f"CRITICAL ERROR: Predefined Volume Group '{VG_NAME}' does not exist.")
        sys.exit(1)
    print(f"SUCCESS: Verified Volume Group '{VG_NAME}' is present.")

app = Flask(__name__)

@app.route('/targetd/rpc', methods=['POST'])
def handle_rpc():
    req = request.get_json()
    if not req or 'method' not in req:
        return jsonify({"error": "Invalid JSON-RPC request"}), 400

    method = req.get("method")
    params = req.get("params", {})

    if method == "vol_create":
        # Integration point for rtslib_fb or targetcli calls
        vol_name = params.get("name")
        return jsonify({
            "result": True, 
            "message": f"Volume {vol_name} processed using VG {VG_NAME}"
        })

    return jsonify({"error": f"Method '{method}' not supported"}), 404

if __name__ == '__main__':
    verify_environment()
    app.run(host=API_HOST, port=API_PORT)
