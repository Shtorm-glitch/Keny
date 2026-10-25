"""Exercise the release binary's real Naive outbound and TLS with a 1 MiB payload.

Local isolated ports only; no production credentials or OS proxy/TUN changes.
KENY_TEST_RUNNER=qemu-aarch64 allows testing the ARM64 release on x86 Linux.
"""
import hashlib
import http.client
import http.server
import json
import os
from pathlib import Path
import shlex
import socket
import ssl
import subprocess
import sys
import tempfile
import threading
import time


def main():
    command = shlex.split(os.environ.get('KENY_TEST_RUNNER', '')) + [str(Path(sys.argv[1]).resolve())]
    version = subprocess.check_output(command + ['version'], text=True)
    assert 'with_naive_outbound' in version and 'with_musl' in version, version
    print(version)
    payload = bytes(range(256)) * 4096

    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            self.send_response(200)
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def log_message(self, *args):
            pass

    processes = []
    server = None
    with tempfile.TemporaryDirectory(prefix='keny-naive-test-') as directory:
        root = Path(directory)
        def openssl(*args):
            subprocess.run(['openssl', *args], cwd=root, check=True, capture_output=True)
        openssl('req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
                '-subj', '/CN=Keny Test CA', '-keyout', 'ca.key', '-out', 'ca.pem')
        openssl('req', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=localhost',
                '-keyout', 'server.key', '-out', 'server.csr')
        (root / 'extensions').write_text('subjectAltName=DNS:localhost\nextendedKeyUsage=serverAuth\n')
        openssl('x509', '-req', '-in', 'server.csr', '-CA', 'ca.pem', '-CAkey', 'ca.key',
                '-CAcreateserial', '-days', '1', '-extfile', 'extensions', '-out', 'server.pem')
        cert, key, ca = [str(root / p) for p in ['server.pem', 'server.key', 'ca.pem']]
        config = {
            'log': {'level': 'warn'},
            'inbounds': [
                {'type': 'mixed', 'tag': 'client', 'listen': '127.0.0.1', 'listen_port': 19080},
                {'type': 'naive', 'tag': 'server', 'network': 'tcp', 'listen': '127.0.0.1',
                 'listen_port': 19443, 'users': [{'username': 'test', 'password': 'local-test'}],
                 'tls': {'enabled': True, 'certificate_path': cert, 'key_path': key}},
            ],
            'outbounds': [
                {'type': 'direct', 'tag': 'direct'},
                {'type': 'naive', 'tag': 'naive', 'server': '127.0.0.1', 'server_port': 19443,
                 'username': 'test', 'password': 'local-test',
                 'tls': {'enabled': True, 'server_name': 'localhost', 'certificate_path': ca}},
            ],
            'route': {'rules': [{'inbound': ['client'], 'action': 'route', 'outbound': 'naive'}],
                      'final': 'direct'},
        }
        path = root / 'config.json'
        path.write_text(json.dumps(config))
        subprocess.run(command + ['check', '-c', str(path)], check=True, timeout=60)
        try:
            server = http.server.ThreadingHTTPServer(('127.0.0.1', 18080), Handler)
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            context.load_cert_chain(cert, key)
            server.socket = context.wrap_socket(server.socket, server_side=True)
            threading.Thread(target=server.serve_forever, daemon=True).start()
            process = subprocess.Popen(command + ['run', '-c', str(path)])
            processes.append(process)
            for _ in range(300):
                if process.poll() is not None:
                    raise RuntimeError('Keny exited before test')
                try:
                    with socket.create_connection(('127.0.0.1', 19080), timeout=0.2):
                        break
                except OSError:
                    time.sleep(0.1)
            else:
                raise TimeoutError('Keny listener startup timed out')
            connection = http.client.HTTPSConnection('127.0.0.1', 19080, timeout=90,
                                                      context=ssl.create_default_context(cafile=ca))
            connection.set_tunnel('localhost', 18080)
            connection.request('GET', '/')
            response = connection.getresponse()
            received = response.read()
            assert response.status == 200
            assert received == payload
            connection.close()
            print('PASS: Naive outbound, verified TLS, 1 MiB payload SHA256', hashlib.sha256(received).hexdigest())
        finally:
            for process in processes:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
            if server:
                server.shutdown()
                server.server_close()


if __name__ == '__main__':
    main()
