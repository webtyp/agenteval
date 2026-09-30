"""Probe decider-4b through llama-server, reproducing decider.prompt.build (plain layout, <=10 options).

prompt ids = tok("Context:\n" + state) + tok("\n\nQuestion: q\nOptions:\n(A) a\n(B) b\nAnswer: (")
The answer is the next-token distribution restricted to the option letters, re-tempered by T.
"""
import json, math, sys, time, urllib.request

URL = "http://127.0.0.1:8090"  # llama-server running decider-4b, see docs/JUDGE.md
LETTERS = "ABCDEFGHIJ"
T_CHOICE = 1.11


def post(path, body):
    req = urllib.request.Request(URL + path, json.dumps(body).encode(), {"Content-Type": "application/json"})
    return json.load(urllib.request.urlopen(req))


def tok(text):
    return post("/tokenize", {"content": text, "add_special": False})["tokens"]


_letter_ids = []


def letter_ids():
    if not _letter_ids:
        _letter_ids.extend(tok(L)[0] for L in LETTERS)
    return _letter_ids


def decide(state, question, options, T=T_CHOICE):
    ids = tok("Context:\n" + state)
    lines = "\n\nQuestion: " + question + "\nOptions:" + "".join(f"\n({LETTERS[j]}) {o}" for j, o in enumerate(options)) + "\nAnswer: ("
    ids += tok(lines)
    t0 = time.time()
    r = post("/completion", {"prompt": ids, "n_predict": 1, "n_probs": 64, "temperature": 0,
                             "cache_prompt": False, "post_sampling_probs": False})
    dt = time.time() - t0
    top = r["completion_probabilities"][0]["top_logprobs"]
    lp = {t["id"]: t["logprob"] for t in top}
    z = [lp.get(letter_ids()[j], -1e9) / T for j in range(len(options))]
    m = max(z)
    p = [math.exp(v - m) for v in z]
    s = sum(p)
    p = [v / s for v in p]
    missing = [options[j] for j in range(len(options)) if letter_ids()[j] not in lp]
    j = max(range(len(options)), key=lambda i: p[i])
    return {"choice": options[j], "confidence": round(p[j], 3),
            "probs": {o: round(v, 3) for o, v in zip(options, p)},
            "letter_mass": round(sum(math.exp(lp[letter_ids()[i]]) for i in range(len(options)) if letter_ids()[i] in lp), 3),
            "missing": missing, "ms": int(dt * 1000), "tokens": len(ids)}


if __name__ == "__main__":
    for path in sys.argv[1:]:
        for c in json.load(open(path)):
            out = decide(c["state"], c["question"], c["options"], c.get("T", T_CHOICE))
            ok = out["choice"] == c["expect"]
            print(("OK  " if ok else "FAIL") + f" {c['name']}: {json.dumps(out, ensure_ascii=False)}")
