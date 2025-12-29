import sys
import re
import json
import os
import datetime
import argparse

def parse_benchmarks(content):
    go_results = {}
    cpp_results = {}

    # Split content into Go and C++ sections
    # This logic depends on the output format of run-benchmarks.sh
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
                # Format: BenchmarkName-8 1000000 12.3 ns/op
                name = parts[0].replace('Benchmark', '').replace('-20', '').replace('-8', '').replace('-16', '').replace('-32', '')
                # Remove trailing numbers like -8 if they exist (GOMAXPROCS)
                name = re.sub(r'-\d+$', '', name)

                try:
                    if 'ns/op' in parts:
                        ns_per_op_idx = parts.index('ns/op')
                        ns_per_op = float(parts[ns_per_op_idx - 1])
                        go_results[name] = ns_per_op
                except (ValueError, IndexError):
                    pass

    # Parse C++ results
    # Format: BenchmarkName 1000000 12.3 ns/op
    for line in cpp_section.split('\n'):
        line = line.strip()
        if line.startswith('Benchmark') and 'Iterations' not in line:
            parts = re.split(r'\s+', line)
            if len(parts) >= 3:
                name = parts[0].replace('Benchmark', '')
                try:
                    # Assuming format: Name Iterations Time ns/op
                    # or: Name Time ns/op
                    # The previous script assumed index 2. Let's be more flexible.
                    # C++ output from previous turn: "BenchmarkVarDecodeUint64 100000000 9.45 ns/op"
                    # parts: ['BenchmarkVarDecodeUint64', '100000000', '9.45', 'ns/op']
                    if 'ns/op' in parts:
                        ns_idx = parts.index('ns/op')
                        ns_per_op = float(parts[ns_idx - 1])
                        cpp_results[name] = ns_per_op
                except (ValueError, IndexError):
                    pass

    return go_results, cpp_results

def generate_report(go_results, cpp_results, timestamp):
    # Define comparison groups (Go name -> C++ name)
    # This mapping needs to be maintained as benchmarks are added
    comparisons = [
        # Varint
        ('VarEncodeUint64', 'VarEncodeUint64'),
        ('VarDecodeUint64', 'VarDecodeUint64'),
        ('VarEncodeInt64', 'VarEncodeInt64'),
        ('VarDecodeInt64', 'VarDecodeInt64'),

        # Basic types
        ('SerializeUInt8', 'SerializeUInt8'),
        ('SerializeUInt8Reuse', 'SerializeUInt8Reuse'),
        ('DeserializeUInt8', 'DeserializeUInt8'),
        ('SerializeFloat32', 'SerializeFloat32'),
        ('SerializeFloat32Reuse', 'SerializeFloat32Reuse'),
        ('DeserializeFloat32', 'DeserializeFloat32'),

        # UInt32 variants
        ('SerializeUInt32/Large', 'SerializeUInt32/Large'),
        ('SerializeUInt32/LargeReuse', 'SerializeUInt32Reuse/Large'),
        ('DeserializeUInt32/Large', 'DeserializeUInt32/Large'),

        # Strings
        ('SerializeString/Short', 'SerializeString/Short'),
        ('SerializeString/Medium', 'SerializeString/Medium'),
        ('DeserializeString/Short', 'DeserializeString/Short'),
        ('DeserializeString/Medium', 'DeserializeString/Medium'),
        ('DeserializeString/Long', 'DeserializeString/Long'),

        # UUID and Time
        ('SerializeUUID', 'SerializeUUID'),
        ('DeserializeUUID', 'DeserializeUUID'),
        ('SerializeTime', 'SerializeTimestamp'),
        ('DeserializeTime', 'DeserializeTimestamp'),

        # Byte arrays
        ('WriteBytesAligned', 'WriteBytesAligned'),
        ('WriteBytesAlignedReuse', 'WriteBytesAlignedReuse'),
        ('WriteBytesUnaligned', 'WriteBytesUnaligned'),
        ('WriteBytesUnalignedReuse', 'WriteBytesUnalignedReuse'),
        ('ReadBytesAligned', 'ReadBytesAligned'),
        ('ReadBytesUnaligned', 'ReadBytesUnaligned'),

        # Messages
        ('GeneratedMessageSmall/Serialize', 'GeneratedMessageSmall/Serialize'),
        ('GeneratedMessageSmall/Deserialize', 'GeneratedMessageSmall/Deserialize'),
        ('GeneratedMessageEcho/Request/Serialize', 'GeneratedMessageEcho/Request/Serialize'),
        ('GeneratedMessageEcho/Request/Deserialize', 'GeneratedMessageEcho/Request/Deserialize'),
        ('GeneratedMessageEcho/Response/Serialize', 'GeneratedMessageEcho/Response/Serialize'),
        ('GeneratedMessageEcho/Response/Deserialize', 'GeneratedMessageEcho/Response/Deserialize'),
        ('GeneratedMessageProcess/Request/Serialize', 'GeneratedMessageProcess/Request/Serialize'),
        ('GeneratedMessageProcess/Request/Deserialize', 'GeneratedMessageProcess/Request/Deserialize'),
        ('GeneratedMessageProcess/Response/Serialize', 'GeneratedMessageProcess/Response/Serialize'),
        ('GeneratedMessageProcess/Response/Deserialize', 'GeneratedMessageProcess/Response/Deserialize'),
        ('GeneratedMessageNested/Serialize', 'GeneratedMessageNested/Serialize'),
        ('GeneratedMessageNested/Deserialize', 'GeneratedMessageNested/Deserialize'),
        ('GeneratedMessageLargePayload/1KB/Serialize', 'GeneratedMessageLargePayload/1KB/Serialize'),
        ('GeneratedMessageLargePayload/1KB/Deserialize', 'GeneratedMessageLargePayload/1KB/Deserialize'),
        ('GeneratedMessageLargePayload/10KB/Serialize', 'GeneratedMessageLargePayload/10KB/Serialize'),
        ('GeneratedMessageLargePayload/10KB/Deserialize', 'GeneratedMessageLargePayload/10KB/Deserialize'),
    ]

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
