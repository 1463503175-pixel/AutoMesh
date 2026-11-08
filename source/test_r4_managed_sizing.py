# coding: utf-8
"""R4 ACT mock regression. Real 2024 R2 verification still required."""
import json
import pathlib
import tempfile
from test_existing_named_sizing_v103 import ExistingNamedSizingTests
from test_editable_groups import MockSizing


def scene():
    h=ExistingNamedSizingTests()
    model,fx=h.setup_scene()
    directory=tempfile.TemporaryDirectory()
    snap=h.inspect(fx,directory.name)
    return directory, model,fx,snap


def test_disable_all_managed_sizings_then_reactivate_without_duplicates():
    d,m,f,s=scene()
    with d:
        first=f.run(d.name,'publish_existing_sizing',2,s,{'existing_named_sizes':{'AM_Hole_01':1.8}})
        assert first['status']=='ok',first.get('error')
        control=m.Mesh.Children[0]
        all_disabled=f.run(d.name,'publish_existing_sizing',3,s,{'existing_named_sizes':{}})
        assert all_disabled['status']=='ok',all_disabled.get('error')
        assert all_disabled['summary']['suppressed_sizing_count']==1
        assert control.Suppressed is True
        assert len(m.Mesh.Children)==1
        reactivated=f.run(d.name,'publish_existing_sizing',4,s,{'existing_named_sizes':{'AM_Hole_01':1.2}})
        assert reactivated['status']=='ok',reactivated.get('error')
        assert control.Suppressed is False
        assert control.ElementSize=='1.2 [mm]'
        assert len(m.Mesh.Children)==1


def test_empty_without_proof_is_rejected():
    d,m,f,s=scene()
    with d:
        r=f.run(d.name,'publish_existing_sizing',2,s,{'existing_named_sizes':{}})
        assert r['status']=='error'
        assert 'no verified AutoMesh-owned Sizing' in r['error']
        assert not m.Mesh.Children


def test_manual_sizing_not_suppressed_even_if_formerly_reused():
    d,m,f,s=scene()
    with d:
        ns=m.NamedSelections.Children[2]
        manual=m.Mesh.AddSizing()
        manual.Name='AutoMesh_Hole_Old' # legacy AutoMesh-looking != verified managed identity
        manual.NamedSelection=ns
        manual.ElementSize='2 [mm]'
        published=f.run(d.name,'publish_existing_sizing',2,s,{'existing_named_sizes':{'AM_Hole_01':1.7}})
        assert published['status']=='ok',published.get('error')
        assert published['summary']['existing_named_sizing'][0]['sizing_status']=='preserved_manual_unmanaged'
        disabled=f.run(d.name,'publish_existing_sizing',3,s,{'existing_named_sizes':{}})
        assert disabled['status']=='error'
        assert manual.Suppressed is False
        assert len(m.Mesh.Children)==1


def test_reactivation_failure_rolls_back_old_ownership_and_suppression():
    d,m,f,s=scene()
    with d:
        good=f.run(d.name,'publish_existing_sizing',2,s,{'existing_named_sizes':{'AM_Hole_01':1.5}})
        assert good['status']=='ok',good.get('error')
        obj=m.Mesh.Children[0]
        prev=pathlib.Path(d.name,'editable_manifest.json').read_bytes()
        # Prevent suppressing via a real read-only property in a mock subclass.
        class ReadOnlySuppress(MockSizing):
            @property
            def Suppressed(self): return False
            @Suppressed.setter
            def Suppressed(self,v): raise RuntimeError('property is read-only')
        replacement=ReadOnlySuppress.__new__(ReadOnlySuppress)
        replacement.__dict__.update(obj.__dict__)
        replacement.Name=obj.Name
        m.Mesh.Children[0]=replacement
        failed=f.run(d.name,'publish_existing_sizing',3,s,{'existing_named_sizes':{}})
        assert failed['status']=='error'
        assert 'rollback=' in failed['error']
        assert pathlib.Path(d.name,'editable_manifest.json').read_bytes()==prev
        assert len(m.Mesh.Children)==1


def test_diagnose_quality_gate_is_string_not_json_object():
    d,m,f,s=scene()
    with d:
        m.Mesh.Elements=25
        diag=f.run(d.name,'diagnose',2,s)
        assert diag['status']=='ok',diag.get('error')
        assert isinstance(diag['summary']['quality_gate'],str)
        assert diag['summary']['quality_status'] in ('pass','warning','fail')


def test_manual_control_even_with_exact_generated_name_is_not_claimed():
    d,m,f,s=scene()
    with d:
        ns=m.NamedSelections.Children[2]
        import hashlib
        name="AM_Hole_01"
        digest=hashlib.sha1(name.encode("utf8")).hexdigest()[:10]
        expected_name="AutoMesh_NS_%s_%s"%(name,digest)
        c=m.Mesh.AddSizing()
        c.Name=expected_name
        c.NamedSelection=ns
        c.ElementSize='2.4 [mm]'
        r=f.run(d.name,'publish_existing_sizing',2,s,{'existing_named_sizes':{'AM_Hole_01':1.7}})
        assert r['status']=='ok',r.get('error')
        assert r['summary']['existing_named_sizing'][0]['sizing_status']=='preserved_manual_unmanaged'
        manifest=json.loads(pathlib.Path(d.name,'editable_manifest.json').read_text())
        assert manifest['prepared']==[], manifest
        attempted=f.run(d.name,'publish_existing_sizing',3,s,{'existing_named_sizes':{}})
        assert attempted['status']=='error',attempted
        assert c.Suppressed is False
        assert c.ElementSize=='2.4 [mm]'
