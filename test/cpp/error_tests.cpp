#include <acutest.h>
#include <string>
#include <vector>
#include <fstream>

#include "scg/error.h"
#include "scg/serialize.h"
#include "scg/writer.h"
#include "scg/reader.h"
#include "scg/macro.h"

using scg::error::Error;
using scg::serialize::bit_size;
using scg::serialize::serialize;
using scg::serialize::deserialize;

void test_error_default_constructor()
{
	Error err;

	TEST_CHECK(!err);
	TEST_CHECK(err == nullptr);
	TEST_CHECK(nullptr == err);
	TEST_CHECK(err.message() == "");
}

void test_error_nullptr_constructor()
{
	Error err(nullptr);

	TEST_CHECK(!err);
	TEST_CHECK(err == nullptr);
	TEST_CHECK(err.message() == "");
}

void test_error_cstring_constructor()
{
	const char* msg = "This is an error message";
	Error err(msg);

	TEST_CHECK(err);
	TEST_CHECK(err != nullptr);
	TEST_CHECK(nullptr != err);
	TEST_CHECK(err.message() == msg);
	TEST_CHECK(err.message() == "This is an error message");
}

void test_error_cstring_constructor_empty()
{
	const char* msg = "";
	Error err(msg);

	TEST_CHECK(!err);  // Empty string should create a null error
	TEST_CHECK(err == nullptr);
	TEST_CHECK(err.message() == "");
}

void test_error_cstring_constructor_nullptr()
{
	const char* msg = nullptr;
	Error err(msg);

	TEST_CHECK(!err);
	TEST_CHECK(err == nullptr);
	TEST_CHECK(err.message() == "");
}

void test_error_string_constructor()
{
	std::string msg = "String error message";
	Error err(msg);

	TEST_CHECK(err);
	TEST_CHECK(err != nullptr);
	TEST_CHECK(err.message() == msg);
	TEST_CHECK(err.message() == "String error message");
}

void test_error_string_constructor_empty()
{
	std::string msg = "";
	Error err(msg);

	TEST_CHECK(!err);  // Empty string should create a null error
	TEST_CHECK(err == nullptr);
	TEST_CHECK(err.message() == "");
}

void test_error_copy_constructor()
{
	Error err1("Original error");
	Error err2(err1);

	TEST_CHECK(err1);
	TEST_CHECK(err2);
	TEST_CHECK(err1.message() == err2.message());
	TEST_CHECK(err1.message() == "Original error");
	TEST_CHECK(err2.message() == "Original error");
	TEST_CHECK(err1 == err2);
}

void test_error_copy_constructor_null()
{
	Error err1;
	Error err2(err1);

	TEST_CHECK(!err1);
	TEST_CHECK(!err2);
	TEST_CHECK(err1.message() == "");
	TEST_CHECK(err2.message() == "");
	TEST_CHECK(err1 == err2);
}

void test_error_move_constructor()
{
	Error err1("Move me");
	Error err2(std::move(err1));

	TEST_CHECK(!err1);
	TEST_CHECK(err2);
	TEST_CHECK(err1.message() == "");
	TEST_CHECK(err2.message() == "Move me");
}

void test_error_move_constructor_null()
{
	Error err1;
	Error err2(std::move(err1));

	TEST_CHECK(!err1);
	TEST_CHECK(!err2);
	TEST_CHECK(err1.message() == "");
	TEST_CHECK(err2.message() == "");
}

void test_error_copy_assignment()
{
	Error err1("First error");
	Error err2("Second error");

	TEST_CHECK(err1.message() == "First error");
	TEST_CHECK(err2.message() == "Second error");

	err2 = err1;

	TEST_CHECK(err1);
	TEST_CHECK(err2);
	TEST_CHECK(err1.message() == "First error");
	TEST_CHECK(err2.message() == "First error");
	TEST_CHECK(err1 == err2);
}

void test_error_copy_assignment_null_to_value()
{
	Error err1;
	Error err2("Has value");

	TEST_CHECK(!err1);
	TEST_CHECK(err2);

	err2 = err1;

	TEST_CHECK(!err1);
	TEST_CHECK(!err2);
	TEST_CHECK(err1.message() == "");
	TEST_CHECK(err2.message() == "");
}

void test_error_copy_assignment_value_to_null()
{
	Error err1("Has value");
	Error err2;

	TEST_CHECK(err1);
	TEST_CHECK(!err2);

	err2 = err1;

	TEST_CHECK(err1);
	TEST_CHECK(err2);
	TEST_CHECK(err1.message() == "Has value");
	TEST_CHECK(err2.message() == "Has value");
}

void test_error_copy_assignment_self()
{
	Error err1("Self assign");

#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wself-assign-overloaded"
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wself-assign-overloaded"
	err1 = err1;
#pragma GCC diagnostic pop
#pragma clang diagnostic pop

	TEST_CHECK(err1);
	TEST_CHECK(err1.message() == "Self assign");
}

void test_error_move_assignment()
{
	Error err1("First error");
	Error err2("Second error");

	TEST_CHECK(err1.message() == "First error");
	TEST_CHECK(err2.message() == "Second error");

	err2 = std::move(err1);

	TEST_CHECK(!err1);
	TEST_CHECK(err2);
	TEST_CHECK(err1.message() == "");
	TEST_CHECK(err2.message() == "First error");
}

void test_error_move_assignment_null_to_value()
{
	Error err1;
	Error err2("Has value");

	TEST_CHECK(!err1);
	TEST_CHECK(err2);

	err2 = std::move(err1);

	TEST_CHECK(!err1);
	TEST_CHECK(!err2);
	TEST_CHECK(err1.message() == "");
	TEST_CHECK(err2.message() == "");
}

void test_error_move_assignment_value_to_null()
{
	Error err1("Has value");
	Error err2;

	TEST_CHECK(err1);
	TEST_CHECK(!err2);

	err2 = std::move(err1);

	TEST_CHECK(!err1);
	TEST_CHECK(err2);
	TEST_CHECK(err1.message() == "");
	TEST_CHECK(err2.message() == "Has value");
}

void test_error_move_assignment_self()
{
	Error err1("Self move");

	// This is UB in general but our implementation should handle it
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wself-move"
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wself-move"
	err1 = std::move(err1);
#pragma GCC diagnostic pop
#pragma clang diagnostic pop

	// After self-move, state is unspecified but should not crash
	// We just check it doesn't crash
	TEST_CHECK(true);
}

void test_error_equality()
{
	Error err1("Same message");
	Error err2("Same message");
	Error err3("Different message");
	Error err4;
	Error err5;

	TEST_CHECK(err1 == err2);
	TEST_CHECK(err1 != err3);
	TEST_CHECK(err2 != err3);
	TEST_CHECK(err4 == err5);
	TEST_CHECK(err4 != err1);
	TEST_CHECK(err4 == nullptr);
	TEST_CHECK(nullptr == err5);
}

void test_error_message_special_chars()
{
	std::string msg = "Error with\nnewline\ttab and 世界 unicode";
	Error err(msg);

	TEST_CHECK(err);
	TEST_CHECK(err.message() == msg);
}

void test_error_message_very_long()
{
	std::string msg(10000, 'x');
	Error err(msg);

	TEST_CHECK(err);
	TEST_CHECK(err.message() == msg);
	TEST_CHECK(err.message().size() == 10000);
}

void test_error_in_vector()
{
	std::vector<Error> errors;

	errors.emplace_back("Error 1");
	errors.emplace_back("Error 2");
	errors.push_back(Error("Error 3"));
	errors.emplace_back();

	TEST_CHECK(errors.size() == 4);
	TEST_CHECK(errors[0].message() == "Error 1");
	TEST_CHECK(errors[1].message() == "Error 2");
	TEST_CHECK(errors[2].message() == "Error 3");
	TEST_CHECK(!errors[3]);

	// Test that copying vector works
	std::vector<Error> copied = errors;
	TEST_CHECK(copied.size() == 4);
	TEST_CHECK(copied[0].message() == "Error 1");
	TEST_CHECK(copied[1].message() == "Error 2");
	TEST_CHECK(copied[2].message() == "Error 3");
	TEST_CHECK(!copied[3]);
}

void test_error_return_value()
{
	auto make_error = [](bool should_error) -> Error {
		if (should_error) {
			return Error("Something went wrong");
		}
		return nullptr;
	};

	Error err1 = make_error(true);
	Error err2 = make_error(false);

	TEST_CHECK(err1);
	TEST_CHECK(err1.message() == "Something went wrong");
	TEST_CHECK(!err2);
	TEST_CHECK(err2.message() == "");
}

void test_error_bool_conversion_in_conditionals()
{
	Error err1("Has error");
	Error err2;

	if (err1) {
		TEST_CHECK(true);
	} else {
		TEST_CHECK(false);
	}

	if (!err2) {
		TEST_CHECK(true);
	} else {
		TEST_CHECK(false);
	}

	TEST_CHECK(err1 ? true : false);
	TEST_CHECK(err2 ? false : true);
}

void test_error_multiple_reassignment()
{
	Error err;

	TEST_CHECK(!err);

	err = Error("First");
	TEST_CHECK(err);
	TEST_CHECK(err.message() == "First");

	err = Error("Second");
	TEST_CHECK(err);
	TEST_CHECK(err.message() == "Second");

	err = Error();
	TEST_CHECK(!err);

	err = Error("Third");
	TEST_CHECK(err);
	TEST_CHECK(err.message() == "Third");
}

// Test struct with Error field for serialization testing
struct TestStructWithError {
	std::string name;
	scg::error::Error error;
};
SCG_SERIALIZABLE_PUBLIC(TestStructWithError, name, error);

void test_error_serialize_deserialize()
{
	// Test serializing a null error
	{
		Error input;

		scg::serialize::Writer writer;
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		Error output("should be cleared");
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(!output);
		TEST_CHECK(output == nullptr);
		TEST_CHECK(output.message() == "");
	}

	// Test serializing an error with a message
	{
		Error input("This is an error");

		scg::serialize::Writer writer;
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		Error output;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(output);
		TEST_CHECK(output != nullptr);
		TEST_CHECK(output.message() == "This is an error");
	}

	// Test serializing an empty string error (should be null)
	{
		Error input("");

		scg::serialize::Writer writer;
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		Error output("should be cleared");
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(!output);
		TEST_CHECK(output == nullptr);
		TEST_CHECK(output.message() == "");
	}
}

void test_error_in_struct_serialization()
{
	// Test with default error
	{
		TestStructWithError input;
		input.name = "test3";

		scg::serialize::Writer writer;
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		TestStructWithError output;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(output.name == "test3");
		TEST_CHECK(!output.error);
		TEST_CHECK(output.error == nullptr);
		TEST_CHECK(output.error.message() == "");
	}

	// Test with null error
	{
		TestStructWithError input;
		input.name = "test1";
		input.error = Error();

		scg::serialize::Writer writer;
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		TestStructWithError output;
		output.error = Error("should be cleared");
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(output.name == "test1");
		TEST_CHECK(!output.error);
		TEST_CHECK(output.error == nullptr);
		TEST_CHECK(output.error.message() == "");
	}

	// Test with error message
	{
		TestStructWithError input;
		input.name = "test2";
		input.error = Error("something went wrong");

		scg::serialize::Writer writer;
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		TestStructWithError output;
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(output.name == "test2");
		TEST_CHECK(output.error);
		TEST_CHECK(output.error != nullptr);
		TEST_CHECK(output.error.message() == "something went wrong");
	}

	// Test with empty string error (should be null)
	{
		TestStructWithError input;
		input.name = "test3";
		input.error = Error("");

		scg::serialize::Writer writer;
		serialize(writer, input);

		scg::serialize::Reader reader(writer.bytes());
		TestStructWithError output;
		output.error = Error("should be cleared");
		auto err = deserialize(output, reader);
		TEST_CHECK(!err);

		TEST_CHECK(output.name == "test3");
		TEST_CHECK(!output.error);
		TEST_CHECK(output.error == nullptr);
		TEST_CHECK(output.error.message() == "");
	}
}

// helper method to reduce redundant test typing
#define TEST(x) {#x, x}

TEST_LIST = {
	TEST(test_error_default_constructor),
	TEST(test_error_nullptr_constructor),
	TEST(test_error_cstring_constructor),
	TEST(test_error_cstring_constructor_empty),
	TEST(test_error_cstring_constructor_nullptr),
	TEST(test_error_string_constructor),
	TEST(test_error_string_constructor_empty),
	TEST(test_error_copy_constructor),
	TEST(test_error_copy_constructor_null),
	TEST(test_error_move_constructor),
	TEST(test_error_move_constructor_null),
	TEST(test_error_copy_assignment),
	TEST(test_error_copy_assignment_null_to_value),
	TEST(test_error_copy_assignment_value_to_null),
	TEST(test_error_copy_assignment_self),
	TEST(test_error_move_assignment),
	TEST(test_error_move_assignment_null_to_value),
	TEST(test_error_move_assignment_value_to_null),
	TEST(test_error_move_assignment_self),
	TEST(test_error_equality),
	TEST(test_error_message_special_chars),
	TEST(test_error_message_very_long),
	TEST(test_error_in_vector),
	TEST(test_error_return_value),
	TEST(test_error_bool_conversion_in_conditionals),
	TEST(test_error_multiple_reassignment),
	TEST(test_error_serialize_deserialize),
	TEST(test_error_in_struct_serialization),

	{ NULL, NULL }
};
