#!/usr/bin/env python3
"""Build AutoMesh's Windows GUI binary and reject any Console subsystem EXE.

Run from any host with Go 1.23+: python build_windows_gui.py
This script does not start AutoMesh, Workbench or Meshing.
"""
import hashlib
import os
from pathlib import Path
import struct
import subprocess


def pe_subsystem(path: Path) -> int:
    with path.open('rb') as stream:
        if stream.read(2) != b'MZ':
            raise ValueError('Not a Windows PE executable (missing MZ)')
        stream.seek(0x3c)
        pe_offset = struct.unpack('<I', stream.read(4))[0]
        stream.seek(pe_offset)
        if stream.read(4) != b'PE\x00\x00':
            raise ValueError('Not a Windows PE executable (missing PE header)')
        file_header = stream.read(20)
        machine, = struct.unpack_from('<H', file_header)
        if machine != 0x8664:
            raise ValueError(f'Expected Windows x64 (machine=0x{machine:x})')
        optional_offset = pe_offset + 24
        stream.seek(optional_offset)
        if struct.unpack('<H', stream.read(2))[0] != 0x20b:
            raise ValueError('Expected PE32+ x64 optional header')
        stream.seek(optional_offset + 68)  # IMAGE_OPTIONAL_HEADER64.Subsystem
        value, = struct.unpack('<H', stream.read(2))
    return value


def build():
    source_dir = Path(__file__).resolve().parent
    output = source_dir.parent / 'AutoMesh.exe'
    staging = source_dir.parent / 'AutoMesh.building.exe'
    env = dict(os.environ, GOOS='windows', GOARCH='amd64', CGO_ENABLED='0')
    try:
        subprocess.run(['go', 'build', '-trimpath', '-ldflags=-H=windowsgui',
                        '-o', str(staging), '.'], cwd=source_dir, env=env,
                       check=True)
        subsystem = pe_subsystem(staging)
        if subsystem != 2:  # IMAGE_SUBSYSTEM_WINDOWS_GUI
            raise RuntimeError(f'Refusing to publish non-GUI executable (subsystem={subsystem})')
        os.replace(staging, output)
    finally:
        staging.unlink(missing_ok=True)
    print('Windows x64 GUI subsystem: OK (2)')
    print('SHA-256:', hashlib.sha256(output.read_bytes()).hexdigest())
    print('Built:', output)


if __name__ == '__main__':
    build()
