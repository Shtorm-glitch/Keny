"""Check the embedded SPA and Manager using disposable loopback services."""
import json
import os
from pathlib import Path
import re
import shlex
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request


def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]


command = shlex.split(os.environ.get('KENY_TEST_RUNNER', '')) + [str(Path(sys.argv[1]).resolve())]
with tempfile.TemporaryDirectory(prefix='keny-admin-test-') as directory:
    root = Path(directory)
    port = free_port()
    config = {
        'log': {'level': 'warn'},
        'services': [
            {'type': 'manager', 'tag': 'manager',
             'database': {'driver': 'sqlite', 'dsn': 'file:manager.db?_pragma=foreign_keys(on)'}},
            {'type': 'admin-panel', 'tag': 'panel', 'listen': '127.0.0.1', 'listen_port': port},
        ],
    }
    config_path = root / 'config.json'
    config_path.write_text(json.dumps(config))
    subprocess.run(command + ['check', '-c', str(config_path)], cwd=root, check=True)
    with (root / 'runtime.log').open('w+') as log:
        process = subprocess.Popen(command + ['run', '-c', str(config_path)], cwd=root,
                                   stdout=log, stderr=subprocess.STDOUT)
        try:
            opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
            deadline = time.monotonic() + 60
            while time.monotonic() < deadline:
                if process.poll() is not None:
                    log.seek(0)
                    raise RuntimeError(log.read())
                try:
                    with opener.open(f'http://127.0.0.1:{port}/', timeout=2) as response:
                        html = response.read().decode()
                        assert response.status == 200 and '<html' in html.lower()
                        assets = re.findall(r'(?:src|href)="(/assets/[^\"]+\.(?:js|css))"', html)
                        assert assets, 'SPA assets missing'
                    for asset in assets:
                        with opener.open(f'http://127.0.0.1:{port}{asset}', timeout=5) as response:
                            assert response.status == 200 and response.read(), asset
                    assert (root / 'manager.db').exists(), 'Manager database missing'
                    print('Admin Panel HTML/assets and Manager SQLite startup: PASS')
                    break
                except OSError:
                    time.sleep(0.2)
            else:
                raise RuntimeError('Admin Panel startup timed out')
        finally:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
