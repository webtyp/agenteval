"""Records what llama-server answers for every judge case, so the Go judge can be tested without a GPU.

Run with decider-4b on port 8090 (see docs/JUDGE.md):
    python3 testdata/judge/record.py testdata/judge/cases_basic.json testdata/judge/cases_hard.json > testdata/judge/recorded.json

For every case it stores the two /tokenize requests with their answers, the /completion answer
exactly as the server returned it, and the result probe.py computes from it. A Go test serves
these answers from a fake server and must reach the same choice and probabilities (within 1e-6).
"""
import json, sys
import probe  # the reference readout, in this same folder

out = []
for path in sys.argv[1:]:
    for c in json.load(open(path)):
        state_text = "Context:\n" + c["state"]
        rest = "\n\nQuestion: " + c["question"] + "\nOptions:" + "".join(
            f"\n({probe.LETTERS[j]}) {o}" for j, o in enumerate(c["options"])) + "\nAnswer: ("
        ids = probe.tok(state_text) + probe.tok(rest)
        completion = probe.post("/completion", {"prompt": ids, "n_predict": 1, "n_probs": 64, "temperature": 0,
                                                "cache_prompt": False, "post_sampling_probs": False})
        result = probe.decide(c["state"], c["question"], c["options"], c.get("T", probe.T_CHOICE))
        result.pop("ms")
        out.append({"case": c, "tokenize": [{"content": state_text, "tokens": probe.tok(state_text)},
                                            {"content": rest, "tokens": probe.tok(rest)}],
                    "completion_probabilities": completion["completion_probabilities"], "result": result})
json.dump({"letters": {L: i for L, i in zip(probe.LETTERS, probe.letter_ids())}, "records": out},
          sys.stdout, ensure_ascii=False, indent=1)
