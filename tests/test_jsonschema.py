import json
import unittest
from pathlib import Path

try:
    import jsonschema
except ImportError:  # pragma: no cover - optional until requirements installed
    jsonschema = None


ROOT = Path(__file__).resolve().parents[1]
SCHEMA_PATH = ROOT / "registry" / "schemas" / "agent-pack.schema.json"
PACKS_PATH = ROOT / "registry" / "packs"
EXAMPLES_PATH = ROOT / "registry" / "schemas" / "examples"


@unittest.skipIf(jsonschema is None, "jsonschema not installed")
class JsonSchemaValidationTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        with SCHEMA_PATH.open(encoding="utf-8") as handle:
            cls.schema = json.load(handle)
        cls.validator = jsonschema.Draft202012Validator(cls.schema)

    def assert_valid_file(self, path: Path):
        with path.open(encoding="utf-8") as handle:
            pack = json.load(handle)
        errors = sorted(self.validator.iter_errors(pack), key=lambda e: e.path)
        messages = [f"{path.name}: {'/'.join(map(str, e.path))} {e.message}" for e in errors]
        self.assertEqual(messages, [])

    def test_registry_packs_match_json_schema(self):
        paths = sorted(PACKS_PATH.glob("*.json"))
        self.assertGreater(len(paths), 0)
        for path in paths:
            with self.subTest(path=path.name):
                self.assert_valid_file(path)

    def test_example_packs_match_json_schema(self):
        paths = sorted(EXAMPLES_PATH.glob("*.json"))
        self.assertGreater(len(paths), 0)
        for path in paths:
            with self.subTest(path=path.name):
                self.assert_valid_file(path)


if __name__ == "__main__":
    unittest.main()
