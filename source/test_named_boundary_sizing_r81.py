# coding: utf-8
"""Mock tests only; no live ANSYS and no generated mesh."""
import pathlib, tempfile, json, unittest
from test_editable_groups import EditableModel, Fixture, MockNamed

class BoundarySizing(unittest.TestCase):
    def inspect(self,fixture,folder,n):
        r=fixture.run(folder,'inspect',n)
        self.assertEqual(r['status'],'ok',r.get('error'))
        return json.loads((pathlib.Path(folder)/'snapshot.json').read_text('utf8'))

    def test_inlet_outlet_independent_sizes_names_and_repeat(self):
        with tempfile.TemporaryDirectory() as d:
            m=EditableModel(); f=Fixture(m); snap=self.inspect(f,d,1)
            r=f.run(d,'publish_existing_sizing',2,snap,{'existing_named_sizes':{'inlet':2.5,'outlet':4.0},
                        'existing_named_scope_ids':{'inlet':[11], 'outlet':[12]}})
            self.assertEqual(r['status'],'ok',r.get('error'))
            self.assertEqual(r['summary']['created_sizing_count'],2)
            self.assertEqual(m.Mesh.count,0)
            self.assertEqual([(x.Name,x.SendToSolver) for x in m.NamedSelections.Children], [('inlet',True),('outlet',True)])
            self.assertEqual(sorted(x.ElementSize for x in m.Mesh.Children),['2.5 [mm]','4 [mm]'])
            # Publishing a different named group must not disable boundary Sizing.
            third=MockNamed('AM_A',[31]);third.SendToSolver=False;m.NamedSelections.Children.append(third)
            snap2=self.inspect(f,d,3)
            r2=f.run(d,'publish_existing_sizing',4,snap2,{'existing_named_sizes':{'AM_A':1.5},
                        'existing_named_scope_ids':{'AM_A':[31]}})
            self.assertEqual(r2['status'],'ok',r2.get('error'))
            self.assertTrue(all(not x.Suppressed for x in m.Mesh.Children))
            # Explicitly republish the inlet only; ensure outlet remains intact.
            r3=f.run(d,'publish_existing_sizing',5,snap2,{'existing_named_sizes':{'inlet':2.2},
                        'existing_named_scope_ids':{'inlet':[11]}})
            self.assertEqual(r3['status'],'ok',r3.get('error'))
            vals={x.NamedSelection.Name:x.ElementSize for x in m.Mesh.Children}
            self.assertEqual(vals['inlet'],'2.2 [mm]')
            self.assertEqual(vals['outlet'],'4 [mm]')
            self.assertEqual(m.Mesh.count,0)

    def test_manual_sizing_kept_and_overlap_blocked(self):
        with tempfile.TemporaryDirectory() as d:
            m=EditableModel(); f=Fixture(m)
            user=m.Mesh.AddSizing(); user.Name='User_Inlet'; user.Location=m.NamedSelections.Children[0];user.ElementSize='1.8 [mm]'
            snap=self.inspect(f,d,1)
            result=f.run(d,'publish_existing_sizing',2,snap,{'existing_named_sizes':{'inlet':2.1},
                        'existing_named_scope_ids':{'inlet':[11]}})
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertEqual(len(m.Mesh.Children),1)
            self.assertEqual(user.ElementSize,'1.8 [mm]')
            other=MockNamed('AM_Overlap',[11]);other.SendToSolver=False;m.NamedSelections.Children.append(other)
            snap2=self.inspect(f,d,3)
            bad=f.run(d,'publish_existing_sizing',4,snap2,{'existing_named_sizes':{'inlet':2.1,'AM_Overlap':1.1},
                     'existing_named_scope_ids':{'inlet':[11],'AM_Overlap':[11]}})
            self.assertEqual(bad['status'],'error');self.assertIn('overlap',bad['error'].lower())
            self.assertEqual(len(m.Mesh.Children),1)
            self.assertEqual(user.ElementSize,'1.8 [mm]')

    def test_boundary_publish_does_not_suppress_previous_automesh_groups(self):
        with tempfile.TemporaryDirectory() as d:
            m=EditableModel();f=Fixture(m)
            extra=MockNamed('AM_Hole',[31]);extra.SendToSolver=False
            m.NamedSelections.Children.append(extra)
            snap=self.inspect(f,d,1)
            first=f.run(d,'publish_existing_sizing',2,snap,{'existing_named_sizes':{'AM_Hole':1.5},
                    'existing_named_scope_ids':{'AM_Hole':[31]}})
            self.assertEqual(first['status'],'ok',first.get('error'))
            original=m.Mesh.Children[0]
            second=f.run(d,'publish_existing_sizing',3,snap,{'existing_named_sizes':{'inlet':2.5},
                    'existing_named_scope_ids':{'inlet':[11]}})
            self.assertEqual(second['status'],'ok',second.get('error'))
            self.assertFalse(original.Suppressed)
            self.assertEqual(original.ElementSize,'1.5 [mm]')
            self.assertEqual(len(m.Mesh.Children),2)
            self.assertEqual(second['summary']['suppressed_sizing_count'],0)

    def test_stale_scope_and_disabled_solver_boundary_refused(self):
        with tempfile.TemporaryDirectory() as d:
            m=EditableModel();f=Fixture(m);snap=self.inspect(f,d,1)
            m.NamedSelections.Children[0].SendToSolver=False
            bad=f.run(d,'publish_existing_sizing',2,snap,{'existing_named_sizes':{'inlet':2.3},
                      'existing_named_scope_ids':{'inlet':[11]}})
            self.assertEqual(bad['status'],'error')
            self.assertEqual(m.Mesh.Children,[])
            m.NamedSelections.Children[0].SendToSolver=True
            m.NamedSelections.Children[0].Location.Ids=[12]
            bad=f.run(d,'publish_existing_sizing',3,snap,{'existing_named_sizes':{'inlet':2.3},
                    'existing_named_scope_ids':{'inlet':[11]}})
            self.assertEqual(bad['status'],'error')
            self.assertEqual(m.Mesh.Children,[])

if __name__=='__main__':unittest.main()
