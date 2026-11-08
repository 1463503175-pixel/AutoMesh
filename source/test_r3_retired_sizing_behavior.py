# coding: utf-8
"""End-to-end mock: disabling an already-published group cannot silently leave it active."""
import json
import pathlib
import tempfile
from test_existing_named_sizing_v103 import ExistingNamedSizingTests


def test_republish_suppresses_retired_sizing_and_keeps_it_inspectable():
    helper=ExistingNamedSizingTests()
    with tempfile.TemporaryDirectory() as folder:
        model,fx=helper.setup_scene()
        snap=helper.inspect(fx,folder)
        first=fx.run(folder,'publish_existing_sizing',2,snap,{'existing_named_sizes':{'AM_Hole_01':1.8}})
        assert first['status']=='ok',first.get('error')
        # User unchecks the old hole group and opts in to a different group.
        second=fx.run(folder,'publish_existing_sizing',3,snap,{'existing_named_sizes':{'4inch':3.5}})
        assert second['status']=='ok',second.get('error')
        retired=second['summary']['retired_managed']
        assert len(retired)==1 and retired[0]['ns']=='AM_Hole_01'
        saved=json.loads((pathlib.Path(folder)/'editable_manifest.json').read_text('utf8'))
        assert saved['retired_managed']==retired
        # Old managed Sizing remains inspectable but must be suppressed.
        old=next(c for c in model.Mesh.Children if c.Name==retired[0]['sizing'])
        assert old.Suppressed is True
        assert second['summary']['suppressed_sizing_count']==1
        assert model.Mesh.count==0


def test_retired_flag_clears_when_group_reenabled():
    helper=ExistingNamedSizingTests()
    with tempfile.TemporaryDirectory() as folder:
        model,fx=helper.setup_scene()
        snap=helper.inspect(fx,folder)
        fx.run(folder,'publish_existing_sizing',2,snap,{'existing_named_sizes':{'AM_Hole_01':1.8}})
        fx.run(folder,'publish_existing_sizing',3,snap,{'existing_named_sizes':{'4inch':3.5}})
        third=fx.run(folder,'publish_existing_sizing',4,snap,{'existing_named_sizes':{'AM_Hole_01':1.8,'4inch':3.5}})
        assert third['status']=='ok',third.get('error')
        assert third['summary']['retired_managed']==[]
        old=next(c for c in model.Mesh.Children if c.Name.startswith('AutoMesh_NS_AM_Hole_01'))
        assert old.Suppressed is False
