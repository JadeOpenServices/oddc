#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
# Contributor tool (debug, read-only): which built-in USB parts of the deployed
# ODDC model carry something that identifies the single unit? The parts come
# from `oddc devices --json` (every device with a usbPort), never from a list
# in this file. Per device it reads, with standard requests only:
#   - every string descriptor (index 1-63) and the Microsoft OS string (0xEE),
#   - the BOS descriptor,
#   - every HID feature report the report descriptor declares (GET_REPORT).
# Nothing is written to a device. Values are never printed or logged: only
# whether something is there, its length, and whether it is a known placeholder.
# Run as root (it needs to read device descriptors), with a Python that has
# pyusb; the log goes to usb-identity-probe.log in the current directory:
#   oddc devices --json > devices.json
#   PY=$(nix build --no-link --print-out-paths --impure --expr \
#     '(builtins.getFlake "nixpkgs").legacyPackages.x86_64-linux.python3.withPackages (p: [p.pyusb])')
#   sudo $PY/bin/python3 -I tools/usb-identity-probe.py devices.json
import json, os, re, subprocess, sys

import usb.core
import usb.util

if os.geteuid() != 0:
    sys.exit("Run as root.")

PLACEHOLDERS = {"0", "00", "000", "0000", "0001", "0x0001", "0x0000", "1234", "12345678",
                "123456789", "ABCD123456789", "200901010001", "SN0001", "0123456789", "1"}

log = open("usb-identity-probe.log", "w")


def say(text):
    print(text, flush=True)
    log.write(text + "\n")
    log.flush()


def describe(value):
    value = value.strip("\x00 ")
    if not value:
        return "empty"
    kind = "placeholder" if value in PLACEHOLDERS or re.fullmatch(r"(.)\1+", value) else "not a known placeholder"
    return f"{len(value)} chars, {kind}"


# The inventory: a file from `oddc devices --json` as the first argument,
# else the installed oddc.
if len(sys.argv) > 1:
    inventory = json.load(open(sys.argv[1]))
else:
    inventory = json.loads(subprocess.run(["oddc", "devices", "--json"], check=True, capture_output=True, text=True).stdout)
say(f"model {inventory['model']}")
for device in inventory["devices"]:
    if not device.get("usbPort"):
        continue
    vid, pid = (int(x, 16) for x in device["deviceId"].split(":"))
    say(f"\n{device['path']} {device['deviceId']} ({device['name']})")
    dev = usb.core.find(idVendor=vid, idProduct=pid)
    if dev is None:
        say("  not present now")
        continue

    serial_index = dev.iSerialNumber
    say(f"  iSerial index: {serial_index or 'none'}")
    try:
        langs = dev.ctrl_transfer(0x80, 6, 0x0300, 0, 255)
        lang = langs[2] | langs[3] << 8 if len(langs) >= 4 else 0x0409
    except usb.core.USBError:
        lang = None
        say("  no string descriptors")
    if lang is not None:
        found = []
        for index in list(range(1, 64)) + [0xEE]:
            try:
                raw = dev.ctrl_transfer(0x80, 6, 0x0300 | index, lang, 255)
            except usb.core.USBError:
                continue
            if len(raw) < 2:
                continue
            text = bytes(raw[2:raw[0]]).decode("utf-16-le", "replace")
            role = {dev.iManufacturer: "manufacturer", dev.iProduct: "product", serial_index: "serial"}.get(index, "other")
            if index == 0xEE:
                role = "Microsoft OS string"
            shown = text if role in ("manufacturer", "product") else describe(text)
            found.append(f"    string {index:#04x} ({role}): {shown}")
        say(f"  string descriptors: {len(found)}")
        for line in found:
            say(line)
    try:
        bos = dev.ctrl_transfer(0x80, 6, 0x0F00, 0, 255)
        say(f"  BOS descriptor: {len(bos)} bytes, {bos[4] if len(bos) > 4 else 0} capabilities")
    except usb.core.USBError:
        say("  BOS descriptor: none")

    for cfg in dev:
        for intf in cfg:
            if intf.bInterfaceClass != 3:
                continue
            try:
                size = 4096
                desc = dev.ctrl_transfer(0x81, 6, 0x2200, intf.bInterfaceNumber, size)
            except usb.core.USBError:
                say(f"  HID interface {intf.bInterfaceNumber}: report descriptor unreadable")
                continue
            # Walk the report descriptor: track Report ID (0x85) and find Feature items (0xB1).
            ids, current, i = set(), 0, 0
            while i < len(desc):
                prefix = desc[i]
                size_code = prefix & 3
                length = 4 if size_code == 3 else size_code
                data = desc[i + 1:i + 1 + length]
                if prefix & 0xFC == 0x84 and data:
                    current = data[0]
                if prefix & 0xFC == 0xB0:
                    ids.add(current)
                i += 1 + length
            say(f"  HID interface {intf.bInterfaceNumber}: feature reports {sorted(ids) or 'none'}")
            for report in sorted(ids):
                try:
                    raw = dev.ctrl_transfer(0xA1, 0x01, 0x0300 | report, intf.bInterfaceNumber, 64)
                    printable = bytes(b for b in raw if 32 <= b < 127)
                    nonzero = sum(1 for b in raw[1:] if b)
                    say(f"    feature {report}: {len(raw)} bytes, {nonzero} non-zero, {len(printable)} printable")
                except usb.core.USBError as e:
                    say(f"    feature {report}: no answer ({e.strerror})")
say("\ndone; values were not recorded")
