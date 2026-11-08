# coding: utf-8
"""Offline verification: no process injection or ANSYS session is invoked."""
import unittest
from pathlib import Path
from assets.wb_server_bootstrap import parse_listening_ports, bootstrap, discover_active_ports

class WorkbenchServerAutostartTests(unittest.TestCase):
    def test_netstat_english_and_chinese_and_ipv6(self):
        lines = '''Active Connections
 TCP  127.0.0.1:10605 0.0.0.0:0 LISTENING 8760
 TCP  [::1]:38195  [::]:0 LISTENING 8760
 TCP  0.0.0.0:5050 0.0.0.0:0 侦听 1010
 TCP  127.0.0.1:38761 127.0.0.1:55555 ESTABLISHED 123
'''
        self.assertEqual(parse_listening_ports(lines),{(10605,8760),(38195,8760),(5050,1010)})

    def test_automatically_find_server_outside_old_fixed_range(self):
        all_ports={(10605,123),(49271,777),(33551,8)}
        names={123:'other.exe',777:'RunWB2.exe',8:'unrelated.exe'}
        result=discover_active_ports(lambda:all_ports, lambda:names,
                                     preferred=10605)
        self.assertEqual(result['process_ports'],[49271])
        self.assertEqual(result['candidate_ports'][0],49271)
        self.assertNotIn(33551,result['candidate_ports'])  # unrelated listeners must not delay StartServer
        self.assertEqual(result['listener_count'],3)

    def test_no_process_info_still_lists_ephemeral_ports(self):
        result=discover_active_ports(lambda:{(49152,99),(12001,55)},
                                     lambda:{}, preferred=10605)
        self.assertEqual(result['process_ports'],[])
        self.assertEqual(set(result['candidate_ports']),{49152,12001})

    def test_chinese_listening_word_variant(self):
        self.assertIn((33444,100),parse_listening_ports(
            ' TCP  127.0.0.1:33444 0.0.0.0:0 正在侦听 100'))

    def test_submit_unavailable_never_retries(self):
        calls=[]
        def invoke(_helper,mode='start'):
            calls.append(mode)
            return {'status':'unavailable','detail':'ambiguous Workbench window'}
        result=bootstrap('fake',invoke=invoke,get_listeners=lambda:set())
        self.assertEqual(calls,['start'])
        self.assertEqual(result['candidate_ports'],[])

    def test_new_listen_port_is_only_a_candidate(self):
        base={(10605,23)}
        states=[base,base,{(10605,23),(38651,91)}]
        def get():return states.pop(0) if states else {(10605,23),(38651,91)}
        result=bootstrap('fake',invoke=lambda *_a,**_k:{'status':'submitted','process_id':91},get_listeners=get,
                         sleep=lambda x:None,deadline=2)
        self.assertEqual(result['status'],'submitted')
        self.assertEqual(result['candidate_ports'],[38651])

    def test_not_starting_new_workbench_or_screen_clicks(self):
        root=Path(__file__).with_name('assets')
        ps=(root/'start_wb_server_uia.ps1').read_text('utf-8')
        py=(root/'wb_server_bootstrap.py').read_text('utf-8')
        self.assertIn("SendWait('{ENTER}')",ps)
        self.assertIn('GetForegroundWindow()',ps)
        self.assertNotIn('from ansys.workbench.core import launch_workbench',py)
        self.assertNotIn('mouse.click',ps.lower())
        self.assertNotIn('screenshot(',ps.lower())

if __name__ == '__main__':unittest.main()
