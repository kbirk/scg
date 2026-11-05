// Downstream-compatibility tests: scg serialization used as a persisted binary
// asset format, extended the way downstream code extends it. Everything here
// uses only long-standing public API (it also compiles against older runtime
// headers), so it guards both lookup behaviour and the exact encoding.

#include <cstdio>
#ifdef SCG_PRINT_GOLDEN
#define TEST_CHECK(x) ((void)(x))
#define TEST_MSG(...) ((void)0)
#else
#include <acutest.h>
#endif

#include <cstdint>
#include <map>
#include <optional>
#include <string>
#include <vector>

#include "scg/serialize.h"
#include "scg/writer.h"
#include "scg/reader.h"
#include "scg/macro.h"
#include "scg/typedef.h"
#include "scg/uuid.h"
#include "scg/timestamp.h"

// ----------------------------------------------------------------------------
// Downstream extension 1: overloads added inside scg::serialize *after* the
// scg headers (here for std::optional<T>).
// ----------------------------------------------------------------------------
namespace scg {
namespace serialize {

template <typename WriterType, typename T>
inline void serialize(WriterType& writer, const std::optional<T>& value)
{
	writer.write(value.has_value());
	if (value) {
		writer.write(*value);
	}
}

template <typename ReaderType, typename T>
inline scg::error::Error deserialize(std::optional<T>& value, ReaderType& reader)
{
	bool has = false;
	auto err = reader.read(has);
	if (err) {
		return err;
	}
	if (!has) {
		value.reset();
		return nullptr;
	}
	T v{};
	err = reader.read(v);
	if (err) {
		return err;
	}
	value = std::move(v);
	return nullptr;
}

}
}

// ----------------------------------------------------------------------------
// Downstream extension 2: ADL overloads in a third-party namespace.
// ----------------------------------------------------------------------------
namespace glm {

struct vec2 {
	float x = 0;
	float y = 0;
	bool operator==(const vec2& o) const { return x == o.x && y == o.y; }
};

template <typename WriterType>
void serialize(WriterType& writer, const vec2& v)
{
	writer.write(v.x);
	writer.write(v.y);
}

template <typename ReaderType>
scg::error::Error deserialize(vec2& v, ReaderType& reader)
{
	auto err = reader.read(v.x);
	if (err) {
		return err;
	}
	return reader.read(v.y);
}

}

// ----------------------------------------------------------------------------
// Asset types using the public macros, nesting the extended types.
// ----------------------------------------------------------------------------
namespace game {

SCG_TYPEDEF(EntityID, uint32_t);

struct Sprite {
	std::string name;
	glm::vec2 origin;
	std::vector<glm::vec2> frames;
	bool operator==(const Sprite& o) const { return name == o.name && origin == o.origin && frames == o.frames; }
};
SCG_SERIALIZABLE_PUBLIC(Sprite, name, origin, frames);

struct Empty {
	bool operator==(const Empty&) const { return true; }
};
SCG_SERIALIZABLE_PUBLIC(Empty);

class Level {
public:
	Level() = default;
	Level(std::string title, std::map<std::string, Sprite> sprites)
		: title_(std::move(title)), sprites_(std::move(sprites)) {}

	std::string title_;
	std::map<std::string, Sprite> sprites_;
	std::optional<int32_t> par_;
	std::vector<std::optional<std::string>> notes_;
	std::map<std::string, std::vector<glm::vec2>> paths_;
	std::vector<Empty> markers_;
	std::map<EntityID, std::optional<glm::vec2>> spawns_;

	bool operator==(const Level& o) const
	{
		return title_ == o.title_ && sprites_ == o.sprites_ && par_ == o.par_ && notes_ == o.notes_ &&
			paths_ == o.paths_ && markers_ == o.markers_ && spawns_ == o.spawns_;
	}

private:
	SCG_SERIALIZABLE_PRIVATE(Level, title_, sprites_, par_, notes_, paths_, markers_, spawns_);
};

struct BaseAsset {
	uint64_t version = 0;
	bool operator==(const BaseAsset& o) const { return version == o.version; }
};
SCG_SERIALIZABLE_PUBLIC(BaseAsset, version);

struct World : BaseAsset {
	std::vector<Level> levels;
	scg::type::uuid id;
	scg::type::timestamp created;
	bool operator==(const World& o) const
	{
		return BaseAsset::operator==(o) && levels == o.levels && id == o.id && created == o.created;
	}
};
SCG_SERIALIZABLE_DERIVED_PUBLIC(World, BaseAsset, levels, id, created);

struct Tagged : BaseAsset {
	bool operator==(const Tagged& o) const { return BaseAsset::operator==(o); }
};
SCG_SERIALIZABLE_DERIVED_PUBLIC(Tagged, BaseAsset);

inline World makeWorld()
{
	World w;
	w.version = 7;
	w.id = scg::type::uuid::fromString("6ba7b810-9dad-41d1-80b4-00c04fd430c8").first;
	w.created = scg::type::timestamp(std::chrono::time_point<std::chrono::system_clock, std::chrono::nanoseconds>(std::chrono::nanoseconds(1700000000123456789LL)));

	Sprite hero{"hero", {1.5f, -2.0f}, {{0, 0}, {16, 0}, {32, 0}}};
	Sprite slime{"slime", {0.25f, 0.75f}, {}};
	Level a("intro", {{"hero", hero}, {"slime", slime}});
	a.par_ = 42;
	a.notes_ = {std::string("first"), std::nullopt, std::string("")};
	a.paths_ = {{"patrol", {{1, 2}, {3, 4}}}, {"none", {}}};
	a.markers_ = {Empty{}, Empty{}, Empty{}};
	a.spawns_ = {{EntityID(3), glm::vec2{5, 6}}, {EntityID(9), std::nullopt}};
	Level b("boss", {});
	w.levels = {a, b};
	return w;
}

}

// Golden bytes of makeWorld(), written by the runtime before the hardening
// changes. The encoding of persisted assets must never change.
static const std::vector<uint8_t> kGoldenWorld = {
#ifndef SCG_PRINT_GOLDEN
#include "downstream_compat_golden.inc"
#endif
};

[[maybe_unused]] static std::string hex(const std::vector<uint8_t>& bytes)
{
	std::string out;
	char buf[4];
	for (size_t i = 0; i < bytes.size(); i++) {
		std::snprintf(buf, sizeof(buf), "%02x", bytes[i]);
		out += buf;
	}
	return out;
}

// Extensions declared after the scg headers resolve through writer.write /
// reader.read, including when nested in std containers and macro structs.
void test_downstream_extensions_round_trip()
{
	auto world = game::makeWorld();

	scg::serialize::Writer writer;
	writer.write(world);
	std::vector<uint8_t> bytes = writer.bytes();

	scg::serialize::Reader reader{std::vector<uint8_t>(bytes)};
	game::World out;
	auto err = reader.read(out);
	TEST_CHECK(err == nullptr);
	TEST_CHECK(out == world);

	// Same through the view types.
	std::vector<uint8_t> viewBytes(1, 0); // older runtimes require a non-empty buffer
	scg::serialize::WriterView view(viewBytes);
	view.write(world);
	TEST_CHECK(viewBytes == bytes);

	scg::serialize::ReaderView rview(bytes.data(), bytes.size());
	game::World out2;
	TEST_CHECK(rview.read(out2) == nullptr);
	TEST_CHECK(out2 == world);

	// Derived struct without extra fields.
	game::Tagged tagged;
	tagged.version = 3;
	scg::serialize::Writer tw;
	tw.write(tagged);
	scg::serialize::Reader tr{std::vector<uint8_t>(tw.bytes())};
	game::Tagged taggedOut;
	TEST_CHECK(tr.read(taggedOut) == nullptr);
	TEST_CHECK(taggedOut == tagged);
}

// The encoding is bit-identical to the golden bytes.
void test_downstream_encoding_is_stable()
{
	auto world = game::makeWorld();
	scg::serialize::Writer writer;
	writer.write(world);
	TEST_CHECK(writer.bytes() == kGoldenWorld);
	TEST_MSG("got %s", hex(writer.bytes()).c_str());

	scg::serialize::Reader reader{std::vector<uint8_t>(kGoldenWorld)};
	game::World out;
	TEST_CHECK(reader.read(out) == nullptr);
	TEST_CHECK(out == world);
}

// SCG_MAP_MEMBER applies a macro to every argument.
#define SCG_TEST_COUNT_ONE(x) +1
void test_macro_map_member()
{
	int n = 0 SCG_MAP_MEMBER(SCG_TEST_COUNT_ONE, a, b, c, d);
	TEST_CHECK(n == 4);
}

#ifdef SCG_PRINT_GOLDEN
// Regenerate the golden bytes (only ever with a known-good runtime):
//   g++ -DSCG_PRINT_GOLDEN ... downstream_compat_tests.cpp && ./a.out > downstream_compat_golden.inc
int main()
{
	auto world = game::makeWorld();
	scg::serialize::Writer writer;
	writer.write(world);
	const auto& b = writer.bytes();
	for (size_t i = 0; i < b.size(); i++) {
		std::printf("0x%02x,%s", b[i], (i % 16 == 15) ? "\n" : " ");
	}
	std::printf("\n");
	return 0;
}
#else
#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_downstream_extensions_round_trip),
	TEST(test_downstream_encoding_is_stable),
	TEST(test_macro_map_member),
	{ NULL, NULL }
};
#endif
