#pragma once

#include <string>
#include <cstring>
#include <functional>
#include <websocketpp/logger/stub.hpp>
#include "scg/logger.h"

namespace scg {
namespace ws {

class LoggerOverride : public websocketpp::log::stub {
public:

	LoggerOverride(websocketpp::log::channel_type_hint::value hint = websocketpp::log::channel_type_hint::access)
		: websocketpp::log::stub(hint)
	{
		type_ = hint == websocketpp::log::channel_type_hint::error ? LogType::ERROR : LogType::ACCESS;
	}

	LoggerOverride(websocketpp::log::level channels, websocketpp::log::channel_type_hint::value hint = websocketpp::log::channel_type_hint::access)
		: websocketpp::log::stub(channels, hint)
	{
		type_ = hint == websocketpp::log::channel_type_hint::error ? LogType::ERROR : LogType::ACCESS;
	}

	void write(websocketpp::log::level channel, std::string const& msg)
	{
		writeInternal(channel, msg.c_str());
	}

	void write(websocketpp::log::level channel, char const* msg)
	{
		writeInternal(channel, msg);
	}

	void registerLoggingFuncs(
		scg::log::LogLevel level,
		std::function<void(std::string)> debugLogger,
		std::function<void(std::string)> infoLogger,
		std::function<void(std::string)> warnLogger,
		std::function<void(std::string)> errorLogger)
	{
		level_ = level;
		if (debugLogger && (level == scg::log::LogLevel::DEBUG)) {
			debugLogger_ = debugLogger;
		}
		if (infoLogger && (level == scg::log::LogLevel::DEBUG || level == scg::log::LogLevel::INFO)) {
			infoLogger_ = infoLogger;
		}
		if (warnLogger && (level == scg::log::LogLevel::DEBUG || level == scg::log::LogLevel::INFO || level == scg::log::LogLevel::WARN)) {
			warnLogger_ = warnLogger;
		}
		if (errorLogger && (level == scg::log::LogLevel::DEBUG || level == scg::log::LogLevel::INFO || level == scg::log::LogLevel::WARN || level == scg::log::LogLevel::ERROR)) {
			errorLogger_ = errorLogger;
		}
	}

	bool dynamic_test(websocketpp::log::level) {
		// let everything through because we filter them in write
		return true;

	}
	constexpr bool static_test(websocketpp::log::level) const {
		// let everything through because we filter them in write
		return true;
	}

private:

	enum class LogType {
		ACCESS,
		ERROR
	};

	void writeInternal(websocketpp::log::level channel, char const* msg)
	{
		std::string formatted;
		if (msg && std::strlen(msg) > 0) {
			formatted = msg;
			formatted[0] = std::toupper(static_cast<unsigned char>(formatted[0]));
		}

		scg::log::LogLevel level = mapToLogLevel(channel);
		switch (level) {
			case scg::log::LogLevel::DEBUG:
				debugLogger_(formatted);
				break;
			case scg::log::LogLevel::INFO:
				infoLogger_(formatted);
				break;
			case scg::log::LogLevel::WARN:
				warnLogger_(formatted);
				break;
			case scg::log::LogLevel::ERROR:
				errorLogger_(formatted);
				break;
			case scg::log::LogLevel::NONE:
				break;
		}
	}

	scg::log::LogLevel mapToLogLevel(websocketpp::log::level level) const
	{
		return type_ == LogType::ACCESS ? mapAccessLevelToLogLevel(level) : mapErrorLevelToLogLevel(level);
	}

	scg::log::LogLevel mapAccessLevelToLogLevel(websocketpp::log::level level) const
	{
		switch (level) {
			case websocketpp::log::alevel::http:
			case websocketpp::log::alevel::frame_header:
			case websocketpp::log::alevel::frame_payload:
				return scg::log::LogLevel::DEBUG;
			case websocketpp::log::alevel::connect:
			case websocketpp::log::alevel::disconnect:
			case websocketpp::log::alevel::fail:
				return scg::log::LogLevel::INFO;
		}
		return scg::log::LogLevel::NONE;
	}

	scg::log::LogLevel mapErrorLevelToLogLevel(websocketpp::log::level level) const
	{
		switch (level) {
			case websocketpp::log::elevel::devel:
				return scg::log::LogLevel::DEBUG;
			case websocketpp::log::elevel::info:
				return scg::log::LogLevel::INFO;
			case websocketpp::log::elevel::warn:
				return scg::log::LogLevel::WARN;
			case websocketpp::log::elevel::rerror:
			case websocketpp::log::elevel::fatal:
				return scg::log::LogLevel::ERROR;
		}
		return scg::log::LogLevel::NONE;
	}

	LogType type_ = LogType::ACCESS;
	scg::log::LogLevel level_ = scg::log::LogLevel::NONE;
	std::function<void(std::string)> debugLogger_ = [](std::string) {};
	std::function<void(std::string)> infoLogger_ = [](std::string) {};
	std::function<void(std::string)> warnLogger_ = [](std::string) {};
	std::function<void(std::string)> errorLogger_ = [](std::string) {};
};

}
}
