# -*- coding: utf-8 -*-
"""Regression for the 2024 R2 ACT read-only/parameterized property error."""
import json
import pathlib
import tempfile
import unittest
from test_editable_groups import EditableModel, Fixture, MockNamed

class ReadOnlyScopingSizing:
    DataModelObjectCategory = 'Sizing'
    def __init__(self, mesh, auto_mode=True, size_readonly=False):
        self.mesh = mesh
        self.Name = ''
        self._mode = 'GeometrySelection'
        self._ns = None
        self._size = None
        self._auto_mode = auto_mode
        self._size_readonly = size_readonly
    @property
    def ScopingMethod(self):
        return self._mode
    @ScopingMethod.setter
    def ScopingMethod(self, value):
        raise ValueError('This property is parameterized and is read-only.')
    @property
    def NamedSelection(self):
        return self._ns
    @NamedSelection.setter
    def NamedSelection(self, ns):
        self._ns = ns
        if self._auto_mode:
            self._mode = 'Component'
    @property
    def ElementSize(self):
        return self._size
    @ElementSize.setter
    def ElementSize(self, value):
        if self._size_readonly:
            raise ValueError('This property is parameterized and is read-only.')
        self._size = value
    @property
    def Location(self):
        return self._ns
    @Location.setter
    def Location(self, ns):
        # API path supported even when ScopingMethod itself is read-only.
        self._ns = ns
        self._mode = 'Component'
    def Delete(self):
        self.mesh.Children.remove(self)

class ParameterizedSizingTests(unittest.TestCase):
    def run_case(self, auto_mode=True, size_readonly=False):
        mesh_model=EditableModel()
        ns=MockNamed('AM_小孔壁',[31]);ns.SendToSolver=False
        mesh_model.NamedSelections.Children.append(ns)
        mesh_model.Mesh.Elements=18098459
        before_names=[x.Name for x in mesh_model.NamedSelections.Children]
        def add():
            obj=ReadOnlyScopingSizing(mesh_model.Mesh,auto_mode=auto_mode,size_readonly=size_readonly)
            mesh_model.Mesh.Children.append(obj)
            return obj
        mesh_model.Mesh.AddSizing=add
        f=Fixture(mesh_model)
        with tempfile.TemporaryDirectory() as d:
            initial=f.run(d,'inspect',1)
            self.assertEqual(initial['status'],'ok',initial.get('error'))
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf-8'))
            result=f.run(d,'publish_existing_sizing',2,snap,{
                'existing_named_sizes':{ns.Name:1.5}
            },allow_existing_mesh_sizing=True)
            self.assertEqual(mesh_model.Mesh.Elements,18098459)
            self.assertEqual(mesh_model.Mesh.count,0)
            self.assertEqual([x.Name for x in mesh_model.NamedSelections.Children],before_names)
            return result,mesh_model

    def test_named_selection_first_avoids_readonly_scoping_setter(self):
        result,model=self.run_case()
        self.assertEqual(result['status'],'ok',result.get('error'))
        self.assertEqual(len(model.Mesh.Children),1)
        sz=model.Mesh.Children[0]
        self.assertEqual(sz.ScopingMethod,'Component')
        self.assertEqual(sz.NamedSelection.Name,'AM_小孔壁')
        self.assertEqual(sz.ElementSize,'1.5 [mm]')

    def test_direct_location_binding_succeeds_even_without_implicit_mode(self):
        result,model=self.run_case(auto_mode=False)
        self.assertEqual(result['status'],'ok',result.get('error'))
        self.assertEqual(model.Mesh.Children[0].NamedSelection.Name,'AM_小孔壁')

    def test_element_size_readonly_reports_exact_stage_and_cleans_new_control(self):
        result,model=self.run_case(size_readonly=True)
        self.assertEqual(result['status'],'error')
        self.assertIn('at ElementSize',result['error'])
        self.assertIn('parameterized and is read-only',result['error'])
        self.assertEqual(model.Mesh.Children,[])

if __name__=='__main__': unittest.main()
