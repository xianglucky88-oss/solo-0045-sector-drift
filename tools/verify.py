import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import urllib.error
import urllib.request


class Service:
    def __init__(self, binary, data):
        self.binary, self.data = binary, data

    def __enter__(self):
        self.proc = subprocess.Popen([self.binary, '-port', '0', '-data', self.data], stderr=subprocess.PIPE, text=True)
        line = self.proc.stderr.readline().strip()
        try:
            self.url = 'http://' + line.split('listening ')[1]
        except IndexError:
            self.__exit__()
            raise RuntimeError(line)
        return self

    def __exit__(self, *exc):
        if self.proc.poll() is None:
            self.proc.terminate()
        try:
            self.proc.wait(timeout=3)
        except subprocess.TimeoutExpired:
            self.proc.kill()
            self.proc.wait()
        self.proc.stderr.close()

    def call(self, path, data=None, headers=None):
        request = urllib.request.Request(self.url + path, data=data, headers=headers or {})
        try:
            with urllib.request.urlopen(request, timeout=15) as response:
                return response.status, response.headers, response.read()
        except urllib.error.HTTPError as error:
            with error:
                return error.code, error.headers, error.read()

    def post(self, value):
        return self.call('/v1/build', json.dumps(value).encode(), {'Content-Type': 'application/json'})

    def demo(self):
        status, _, data = self.call('/v1/example')
        assert status == 200
        return json.loads(data)


def receipt(result):
    status, headers, body = result
    assert status in (200, 201), (status, body)
    row = json.loads(body)
    assert headers['Location'] == row['artifact'] == '/v1/artifacts/' + row['digest']
    return row


def fetch(s, row):
    status, headers, blob = s.call(row['artifact'])
    assert status == 200 and len(blob) == row['bytes']
    assert hashlib.sha256(blob).hexdigest() == row['digest']
    assert headers['ETag'] == '"' + row['digest'] + '"'
    return blob


def contract(s):
    assert s.call('/healthz')[0] == 200
    assert s.call('/v1/build', b'{}')[0] == 415
    json_type = {'Content-Type': 'application/json'}
    assert s.call('/v1/build', b'{"unknown": 1}', json_type)[0] == 400
    assert s.call('/v1/build', b'{} {}', json_type)[0] == 400
    assert s.call('/v1/build', b'{"space":"' + b' '*((3<<20)+1) + b'"}', json_type)[0] == 413
    assert s.call('/v1/artifacts/absent')[0] == 404


def main():
    mode = sys.argv[1] if len(sys.argv) > 1 else 'all'
    if mode not in ('product', 'persistence', 'contract', 'all'):
        raise SystemExit('mode: product | persistence | contract | all')
    with tempfile.TemporaryDirectory(prefix='media-workflow-') as root:
        binary, data = Path(root)/'server', Path(root)/'artifacts'
        subprocess.run(['go','build','-o',str(binary),'./cmd/server'], check=True)
        with Service(binary, data) as s:
            if mode in ('contract', 'all'):
                contract(s)
            if mode in ('product', 'all'):
                check_product(s)
            if mode in ('persistence', 'all'):
                example = s.demo()
                with concurrent.futures.ThreadPoolExecutor(2) as pool:
                    results = list(pool.map(s.post, [example, example]))
                assert [receipt(x) for x in results][0] == [receipt(x) for x in results][1]
                row = receipt(results[0]); stock = fetch(s, row)
                assert not list(data.glob('.staging-*'))
                assert s.call(row['artifact'], headers={'If-None-Match':'"' + row['digest'] + '"'})[0] == 304
                code, headers, prefix = s.call(row['artifact'], headers={'Range':'bytes=0-7'})
                assert code == 206 and prefix == stock[:8]
        if mode in ('persistence', 'all'):
            with Service(binary, data) as s:
                assert fetch(s,row) == stock
                (data/row['digest']).write_bytes(b'corrupted')
                assert s.call(row['artifact'])[0] == 409
    print(mode + ': passed; temporary listener closed')


from copy import deepcopy


def check_product(s):
    spec = s.demo()
    row = receipt(s.post(spec))
    plan = json.loads(fetch(s, row))
    frames = plan['Frames']
    assert len(frames) == 6 and plan['Pinned'] == ['horizon']
    assert [(x['Step'],x['Gate']) for x in plan['Crossings']] == [(2,'ramp'),(4,'chute')]
    assert [(f['Bytes']) for f in frames] == [34,54,54,54,42,42]
    assert any(not c['Admitted'] for c in frames[3]['Decisions'])
    assert [op['Asset'] for op in frames[1]['Operations']] == ['span-mesh']
    assert not frames[2]['Operations']
    assets = {a['ID']:a for a in spec['Assets']}
    loaded = set()
    for frame in frames:
        for op in frame['Operations']:
            asset = op['Asset']
            if op['Action'] == 'evict': loaded.remove(asset)
            else:
                assert asset not in loaded
                loaded.add(asset)
            assert all(set(assets[a]['After']) <= loaded for a in loaded)
            assert sum(assets[a]['Bytes'] for a in loaded) <= plan['Budget']
        assert loaded == set(frame['Resident'])
        assert set(frame['Required']) <= loaded
    assert plan['Usage']['PeakBytes'] == 54 == max(f['Bytes'] for f in frames)
    shuffled = deepcopy(spec)
    shuffled['Assets'].reverse(); shuffled['Gates'].reverse(); shuffled['Sectors'].reverse()
    assert receipt(s.post(shuffled)) == row
    for invalid in ('budget','cycle','gate','direction'):
        broken = deepcopy(spec)
        if invalid == 'budget': broken['Policy']['Budget'] = 20
        if invalid == 'cycle': broken['Assets'][0]['After'] = ['bank-mesh']
        if invalid == 'gate': broken['Route'][1]['Position'] = 40
        if invalid == 'direction': broken['Route'] += [{'Sector':'span','Position':95}]
        status, _, body = s.post(broken)
        assert status == 422 and 'error' in json.loads(body), (invalid, status, body)


if __name__ == '__main__':
    main()
