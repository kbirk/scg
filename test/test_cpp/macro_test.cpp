#include "scg/serialize.h"
#include "scg/reader.h"
#include "scg/writer.h"
#include "scg/macro.h"
#include <iostream>
#include <cassert>

// adl trickery
using scg::serialize::bit_size;
using scg::serialize::serialize;
using scg::serialize::deserialize;

// Test struct with no fields - this was causing the warning
struct Empty {};
SCG_SERIALIZABLE_PUBLIC(Empty);

// Test struct with fields
struct WithFields {
    int x;
    double y;
};
SCG_SERIALIZABLE_PUBLIC(WithFields, x, y);

// Test derived with no extra fields
struct Base {
    int base_val;
};
SCG_SERIALIZABLE_PUBLIC(Base, base_val);

struct DerivedEmpty : Base {};
SCG_SERIALIZABLE_DERIVED_PUBLIC(DerivedEmpty, Base);

// Test derived with extra fields
struct DerivedWithFields : Base {
    double derived_val;
};
SCG_SERIALIZABLE_DERIVED_PUBLIC(DerivedWithFields, Base, derived_val);

int main() {
    // Test Empty
    {
        scg::serialize::Writer writer;
        Empty e1;
        serialize(writer, e1);
        std::cout << "Empty serialized: " << writer.bytes().size() << " bytes" << std::endl;
    }

    // Test WithFields
    scg::serialize::Writer writer;
    WithFields wf1{42, 3.14};
    serialize(writer, wf1);
    std::cout << "WithFields serialized: " << writer.bytes().size() << " bytes" << std::endl;

    scg::serialize::Reader reader(writer.bytes());
    WithFields wf2;
    auto err = deserialize(wf2, reader);
    assert(!err);
    assert(wf2.x == 42);
    assert(wf2.y == 3.14);
    std::cout << "WithFields deserialized: x=" << wf2.x << ", y=" << wf2.y << std::endl;

    // Test DerivedEmpty
    {
        scg::serialize::Writer writer;
        DerivedEmpty de1;
        de1.base_val = 100;
        serialize(writer, de1);
        std::cout << "DerivedEmpty serialized: " << writer.bytes().size() << " bytes" << std::endl;

        scg::serialize::Reader reader(writer.bytes());
        DerivedEmpty de2;
        auto err = deserialize(de2, reader);
        assert(!err);
        assert(de2.base_val == 100);
        std::cout << "DerivedEmpty deserialized: base_val=" << de2.base_val << std::endl;
    }

    // Test DerivedWithFields
    {
        scg::serialize::Writer writer;
        DerivedWithFields dwf1;
        dwf1.base_val = 200;
        dwf1.derived_val = 2.71;
        serialize(writer, dwf1);
        std::cout << "DerivedWithFields serialized: " << writer.bytes().size() << " bytes" << std::endl;

        scg::serialize::Reader reader(writer.bytes());
        DerivedWithFields dwf2;
        auto err = deserialize(dwf2, reader);
        assert(!err);
        assert(dwf2.base_val == 200);
        assert(dwf2.derived_val == 2.71);
        std::cout << "DerivedWithFields deserialized: base_val=" << dwf2.base_val << ", derived_val=" << dwf2.derived_val << std::endl;
    }

    std::cout << "\nAll macro tests passed!" << std::endl;
    return 0;
}
