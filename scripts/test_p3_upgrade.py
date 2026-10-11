import unittest
from check_p3_upgrade import validate_features

class CoverageTest(unittest.TestCase):
    def feature(self, paths):
        return {'id':'fixture', 'source_paths':paths, 'associations':{k:{'status':'pending','note':'needs runtime check'} for k in ['ui','api_dto','schema','config','tests']}}
    def test_missing_frontend_is_rejected(self):
        rows=[{'path':p,'release_delta':True} for p in ['backend/a.go','frontend/a.vue']]
        self.assertTrue(validate_features(rows,[self.feature(['backend/a.go'])]))
        self.assertFalse(validate_features(rows,[self.feature(['backend/a.go','frontend/a.vue'])]))
    def test_duplicate_and_unrelated_assignment_are_rejected(self):
        rows=[{'path':'a','release_delta':True}]
        for paths in [['a','a'],['a','b'],[]]:
            self.assertTrue(validate_features(rows,[self.feature(paths)]))
    def test_missing_schema_slot_is_rejected(self):
        feature=self.feature(['a']); del feature['associations']['schema']
        self.assertTrue(validate_features([{'path':'a','release_delta':True}],[feature]))
if __name__=='__main__': unittest.main()
