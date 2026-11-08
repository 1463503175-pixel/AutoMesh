# coding: utf-8
"""Static regression for Win32 message path; Windows binary must still be built."""
import pathlib
import re
import unittest

S=pathlib.Path(__file__).with_name('main_windows.go').read_text('utf-8')

class WinEventRegression(unittest.TestCase):
    def test_all_critical_buttons_in_command_dispatch_range(self):
        lookup={name:int(number) for name,number in re.findall(r'\b(ID_[A-Z_]+)\s*=\s*(\d+)',S)}
        for n in ('ID_FINE_PREPARE','ID_FINE_GENERATE','ID_EXPORT_MSH','ID_FINE_GROUP_SAVE','ID_FINE_NAMES', 'ID_WARNING_PLAN','ID_WARNING_PUBLISH'):
            self.assertIn(n,lookup)
            self.assertLessEqual(lookup[n],lookup['ID_WARNING_PUBLISH'])
            self.assertIn('case '+n+':',S)
        self.assertIn('if isActionID(id)',S)
        self.assertRegex(S, r'id\s*<=\s*ID_REVIEW_DETAILS')
        for name in ('ID_INIT_RETRY', 'ID_TASK_STATUS'):
            self.assertIn('case '+name+':', S)
        for name in ('initretry', 'taskstatusconn'):
            self.assertIn('a.put("'+name+'"', S)
    def test_scan_populates_group_selection_and_saves_plan(self):
        for required in ('CB_RESETCONTENT','CB_SETCURSEL','GroupOverrides','DisabledGroups',
                         'a.syncGroupEditor()', 'a.saveGroupEditor()'):
            self.assertIn(required,S)
