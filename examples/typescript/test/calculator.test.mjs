import assert from "node:assert/strict";
import test from "node:test";

import { add } from "../dist/calculator.js";

test("add returns the sum", () => {
  assert.equal(add(2, 3), 5);
});
