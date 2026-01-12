import sys
import json
import os
import glob
import argparse

def load_json(filepath):
    with open(filepath, 'r') as f:
        return json.load(f)

def compare_benchmarks(prev_data, curr_data):
    prev_results = prev_data.get("results", {})
    curr_results = curr_data.get("results", {})

    lines = []
    lines.append(f"# Benchmark Comparison")
    lines.append(f"Comparing **{prev_data.get('timestamp')}** vs **{curr_data.get('timestamp')}**")
    lines.append("")
    lines.append("| Benchmark | Previous C++ (ns) | Current C++ (ns) | Delta | Status |")
    lines.append("|-----------|-------------------|------------------|-------|--------|")

    all_keys = set(prev_results.keys()) | set(curr_results.keys())

    for key in sorted(all_keys):
        prev_entry = prev_results.get(key)
        curr_entry = curr_results.get(key)

        if not prev_entry or not curr_entry:
            continue

        prev_cpp = prev_entry.get("cpp", 0)
        curr_cpp = curr_entry.get("cpp", 0)

        if prev_cpp == 0: continue

        delta_pct = ((curr_cpp - prev_cpp) / prev_cpp) * 100

        status = ""
        if delta_pct < -5.0:
            status = "🚀 Improved" # Lower time is better
        elif delta_pct > 5.0:
            status = "⚠️ Regressed"
        else:
            status = "Stable"

        lines.append(f"| {key} | {prev_cpp:.2f} | {curr_cpp:.2f} | {delta_pct:+.2f}% | {status} |")

    return "\n".join(lines)

def main():
    parser = argparse.ArgumentParser(description='Compare last two benchmark results')
    parser.add_argument('--report-dir', required=True, help='Directory containing benchmark JSON reports')
    args = parser.parse_args()

    # Find all JSON files matching the pattern
    files = glob.glob(os.path.join(args.report_dir, "benchmark-*.json"))
    files.sort() # Sort by filename (which includes timestamp)

    if len(files) < 2:
        print("Not enough benchmark history to compare (need at least 2 runs).")
        return

    prev_file = files[-2]
    curr_file = files[-1]

    print(f"Comparing {os.path.basename(prev_file)} with {os.path.basename(curr_file)}...")

    prev_data = load_json(prev_file)
    curr_data = load_json(curr_file)

    comparison_md = compare_benchmarks(prev_data, curr_data)

    curr_md_file = curr_file.replace(".json", ".md")

    if os.path.exists(curr_md_file):
        with open(curr_md_file, 'a') as f:
            f.write("\n\n")
            f.write(comparison_md)
        print(f"Appended comparison to {curr_md_file}")
    else:
        print(comparison_md)

if __name__ == "__main__":
    main()
