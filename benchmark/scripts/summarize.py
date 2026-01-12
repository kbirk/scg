import sys
import re
import json
import os
import datetime
import argparse

def parse_benchmarks(content):
    go_results = {}
    cpp_results = {}

    go_section = ""
    cpp_section = ""

    if 'Go benchmarks completed successfully' in content:
        parts = content.split('Go benchmarks completed successfully')
        go_section = parts[0]
        if len(parts) > 1:
            cpp_section = parts[1]
    elif 'Building C++ benchmarks' in content:
         parts = content.split('Building C++ benchmarks')
         go_section = parts[0]
         if len(parts) > 1:
            cpp_section = parts[1]
    else:
        # Fallback: assume first part is Go, look for C++ marker
        if 'Running C++' in content:
             parts = re.split(r'Running C\+\+ .*benchmarks\.\.\.', content)
             go_section = parts[0]
             cpp_section = "\n".join(parts[1:])
        else:
            go_section = content

    # Parse Go results
    for line in go_section.split('\n'):
        if line.startswith('Benchmark'):
            parts = re.split(r'\s+', line.strip())
            if len(parts) >= 3:
                name = parts[0].replace('Benchmark', '').replace('-20', '').replace('-8', '').replace('-16', '').replace('-32', '')
                name = re.sub(r'-\d+$', '', name)

                try:
                    if 'ns/op' in parts:
                        ns_per_op_idx = parts.index('ns/op')
                        ns_per_op = float(parts[ns_per_op_idx - 1])
                        go_results[name] = ns_per_op
                except (ValueError, IndexError):
                    pass

    for line in cpp_section.split('\n'):
        line = line.strip()
        if line.startswith('Benchmark') and 'Iterations' not in line:
            parts = re.split(r'\s+', line)
            if len(parts) >= 3:
                name = parts[0].replace('Benchmark', '')
                try:
                    if 'ns/op' in parts:
                        ns_idx = parts.index('ns/op')
                        ns_per_op = float(parts[ns_idx - 1])
                        cpp_results[name] = ns_per_op
                except (ValueError, IndexError):
                    pass

    return go_results, cpp_results

def generate_report(go_results, cpp_results, timestamp):
    all_benchmarks = set(go_results.keys()) | set(cpp_results.keys())

    comparisons = []
    for name in sorted(all_benchmarks):
        if name in go_results and name in cpp_results:
            comparisons.append((name, name))
        elif name in go_results:
            cpp_candidate = name.replace('Time', 'Timestamp')
            if cpp_candidate in cpp_results:
                comparisons.append((name, cpp_candidate))
        elif name in cpp_results:
            go_candidate = name.replace('Timestamp', 'Time')
            if go_candidate in go_results:
                comparisons.append((go_candidate, name))

    report_lines = []
    report_lines.append(f"# Benchmark Report - {timestamp}")
    report_lines.append("")
    report_lines.append("| Benchmark | Go (ns/op) | C++ (ns/op) | Ratio (Go/C++) | Status |")
    report_lines.append("|-----------|------------|-------------|----------------|--------|")

    json_data = {
        "timestamp": timestamp,
        "results": {}
    }

    wild_discrepancies = []

    for go_name, cpp_name in comparisons:
        if go_name in go_results and cpp_name in cpp_results:
            go_val = go_results[go_name]
            cpp_val = cpp_results[cpp_name]
            ratio = go_val / cpp_val if cpp_val > 0 else 0.0

            status = ""
            if ratio > 1.1:
                status = "✅ C++ Faster"
            elif ratio < 0.9:
                status = "❌ Go Faster"
            else:
                status = "≈ Parity"

            report_lines.append(f"| {go_name} | {go_val:.2f} | {cpp_val:.2f} | {ratio:.2f}x | {status} |")

            json_data["results"][go_name] = {
                "go": go_val,
                "cpp": cpp_val,
                "ratio": ratio,
                "cpp_name": cpp_name
            }

            if ratio > 5.0:
                wild_discrepancies.append(f"**{go_name}**: Go is {ratio:.2f}x slower than C++")
            elif ratio < 0.2:
                wild_discrepancies.append(f"**{go_name}**: C++ is {1/ratio:.2f}x slower than Go")

    if wild_discrepancies:
        report_lines.append("")
        report_lines.append("## Wild Discrepancies (>5x)")
        for item in wild_discrepancies:
            report_lines.append(f"- {item}")

    return "\n".join(report_lines), json_data

def main():
    parser = argparse.ArgumentParser(description='Summarize benchmark results')
    parser.add_argument('--input', required=True, help='Input file containing raw benchmark output')
    parser.add_argument('--output-dir', required=True, help='Directory to save reports')
    args = parser.parse_args()

    with open(args.input, 'r') as f:
        content = f.read()

    timestamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
    go_results, cpp_results = parse_benchmarks(content)

    md_report, json_data = generate_report(go_results, cpp_results, timestamp)

    # Ensure output directory exists
    os.makedirs(args.output_dir, exist_ok=True)

    # Write Markdown report
    md_filename = os.path.join(args.output_dir, f"benchmark-{timestamp}.md")
    with open(md_filename, 'w') as f:
        f.write(md_report)
    print(f"Generated report: {md_filename}")

    # Write JSON data (for comparison)
    json_filename = os.path.join(args.output_dir, f"benchmark-{timestamp}.json")
    with open(json_filename, 'w') as f:
        json.dump(json_data, f, indent=2)
    print(f"Generated data: {json_filename}")

if __name__ == "__main__":
    main()
