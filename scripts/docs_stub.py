#!/usr/bin/env python3
"""Отдаёт каталог так, как его отдал бы GitHub: коммит ветки и архив дерева.

Нужен ровно для одного: снять витрину бэклога тем же способом, что и остальные экраны, — с тела,
которое отдал настоящий сервер. Сервер за архивом ходит по сети, и без заглушки съёмка требовала бы
живого репозитория, доступа к нему и работающего интернета на машине, где переснимают картинки.

    python3 scripts/docs_stub.py --root scripts/fixtures/docs-source --addr 127.0.0.1:8479

Отвечает на два адреса и ни на что больше:

    GET /repos/<owner>/<name>/commits/<ref>   отпечаток содержимого каталога, строкой
    GET /repos/<owner>/<name>/tarball/<sha>   tar.gz с деревом под каталогом-именем коммита

Отпечаток считается по содержимому, а не берётся с потолка: сервер скачивает архив только когда
коммит изменился, и заглушка, называющая один и тот же коммит после правки фикстуры, отдавала бы
старую картинку — то есть съёмка молча записывала бы прошлое.
"""
import argparse
import hashlib
import io
import os
import pathlib
import tarfile
from http.server import BaseHTTPRequestHandler, HTTPServer


def digest(root: pathlib.Path) -> str:
    accumulator = hashlib.sha256()
    for path in sorted(root.rglob("*")):
        if path.is_file():
            accumulator.update(str(path.relative_to(root)).encode())
            accumulator.update(path.read_bytes())
    return accumulator.hexdigest()[:40]


def archive(root: pathlib.Path, sha: str) -> bytes:
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode="w:gz") as tar:
        for path in sorted(root.rglob("*")):
            if path.is_file():
                tar.add(path, arcname=f"fixture-docs-{sha}/{path.relative_to(root)}")
    return buffer.getvalue()


def serve(root: pathlib.Path, host: str, port: int) -> None:
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):  # noqa: N802 - имя задано базовым классом
            sha = digest(root)
            if "/commits/" in self.path:
                self.answer(sha.encode(), "text/plain")
            elif "/tarball/" in self.path:
                self.answer(archive(root, sha), "application/gzip")
            else:
                self.send_error(404)

        def answer(self, body: bytes, kind: str) -> None:
            self.send_response(200)
            self.send_header("Content-Type", kind)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *_):
            pass

    HTTPServer((host, port), Handler).serve_forever()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True)
    parser.add_argument("--addr", default="127.0.0.1:8479")
    arguments = parser.parse_args()

    root = pathlib.Path(arguments.root).resolve()
    if not root.is_dir():
        raise SystemExit(f"нет каталога {root}")
    host, _, port = arguments.addr.rpartition(":")
    serve(root, host or "127.0.0.1", int(port))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
