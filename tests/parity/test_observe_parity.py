"""Offline checks of the parity checker itself: it must fail what the live gate is there to catch.

Run: python -m unittest discover -s tests/parity
"""

import unittest
from types import SimpleNamespace
from typing import Any

from observe_parity import compare

URL = "https://example.com/form"


def sdk(description: str, states: dict[str, Any] | None) -> Any:
    return SimpleNamespace(
        metadata=SimpleNamespace(url=URL),
        space=SimpleNamespace(description=description),
        element_states=states,
    )


def problems(description: str, states: dict[str, Any], cli_body: str, page: dict[str, Any] | None = None) -> list[str]:
    text = f"Form - {URL}\n\n{cli_body}\n"
    return compare(page or {}, sdk(description, states), text, {"element_states": states})


class ParityCheckerTest(unittest.TestCase):
    def test_merged_state_passes(self) -> None:
        self.assertEqual(
            problems(
                'B1[:]<input type="checkbox"></input>\nI1[:]<input type="password" name="pw"></input>',
                {"B1": {"checked": True}, "I1": {"filled": True}},
                'B1[:]<input type="checkbox" checked></input>\nI1[:]<input type="password" name="pw" filled></input>',
            ),
            [],
        )

    def test_missing_state_fails(self) -> None:
        line = 'B1[:]<input type="checkbox"></input>'
        found = problems(line, {"B1": {"checked": True}}, line)
        self.assertTrue(any("missing `checked`" in p for p in found), found)

    def test_repeated_state_fails(self) -> None:
        found = problems(
            'I1[:]<input type="password" name="pw"></input>',
            {"I1": {"filled": True}},
            'I1[:]<input type="password" name="pw" filled filled></input>',
        )
        self.assertTrue(any("repeats `filled`" in p for p in found), found)

    def test_rewriting_a_line_the_api_wrote_fails(self) -> None:
        found = problems(
            'I1[:]<input type="text" name="q" value="a&#10;b"></input>',
            {"I1": {"value": "a\nb"}},
            'I1[:]<input type="text" name="q" value="a\\nb"></input>',
        )
        self.assertTrue(any("rewrote it" in p for p in found), found)

    def test_changing_more_than_state_fails(self) -> None:
        found = problems('L1[:]<a href="/x">X</a>', {}, 'L1[:]<a href="/y">X</a>')
        self.assertTrue(any("changed" in p for p in found), found)

    def test_a_typed_secret_fails(self) -> None:
        found = problems(
            'I1[:]<input type="password" name="pw"></input>',
            {"I1": {"filled": True}},
            'I1[:]<input type="password" name="pw" filled value="hunter2-secret"></input>',
            page={"secrets": ["hunter2-secret"]},
        )
        self.assertTrue(any("secret" in p for p in found), found)


if __name__ == "__main__":
    unittest.main()
