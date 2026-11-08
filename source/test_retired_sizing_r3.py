import ast
from pathlib import Path

worker=Path(__file__).parent/'assets'/'meshing_worker.py'

def test_worker_contains_explicit_retirement_checks():
    text=worker.read_text(encoding='utf8')
    ast.parse(text)
    assert 'retired_managed' in text
    assert 'Disabled AutoMesh Sizing remains active' in text
    assert 'control.Location = ns' in text
    assert "'ScopingMethod', 'NamedSelection'" not in text[text.index('def _bind_named_sizing'):text.index('def _create_verified_named_sizing')]

