#pragma once

#include <string>

#include <websocketpp/logger/stub.hpp>

namespace scg {
namespace log {

enum class LogLevel {
	DEBUG,
	INFO,
	WARN,
	ERROR,
	NONE
};

struct LoggingConfig {
	LogLevel level = LogLevel::INFO;
	std::function<void(std::string)> debugLogger;
	std::function<void(std::string)> infoLogger;
	std::function<void(std::string)> warnLogger;
	std::function<void(std::string)> errorLogger;
};

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
		LogLevel level,
		std::function<void(std::string)> debugLogger,
		std::function<void(std::string)> infoLogger,
		std::function<void(std::string)> warnLogger,
		std::function<void(std::string)> errorLogger)
	{
		level_ = level;
		if (debugLogger && (level == LogLevel::DEBUG)) {
			debugLogger_ = debugLogger;
		}
		if (infoLogger && (level == LogLevel::DEBUG || level == LogLevel::INFO)) {
			infoLogger_ = infoLogger;
		}
		if (warnLogger && (level == LogLevel::DEBUG || level == LogLevel::INFO || level == LogLevel::WARN)) {
			warnLogger_ = warnLogger;
		}
		if (errorLogger && (level == LogLevel::DEBUG || level == LogLevel::INFO || level == LogLevel::WARN || level == LogLevel::ERROR)) {
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

		LogLevel level = mapToLogLevel(channel);
		switch (level) {
			case LogLevel::DEBUG:
				debugLogger_(formatted);
				break;
			case LogLevel::INFO:
				infoLogger_(formatted);
				break;
			case LogLevel::WARN:
				warnLogger_(formatted);
				break;
			case LogLevel::ERROR:
				errorLogger_(formatted);
				break;
			case LogLevel::NONE:
				break;
		}
	}

	LogLevel mapToLogLevel(websocketpp::log::level level) const
	{
		return type_ == LogType::ACCESS ? mapAccessLevelToLogLevel(level) : mapErrorLevelToLogLevel(level);
	}

	LogLevel mapAccessLevelToLogLevel(websocketpp::log::level level) const
	{
		switch (level) {
			case websocketpp::log::alevel::http:
			case websocketpp::log::alevel::frame_header:
			case websocketpp::log::alevel::frame_payload:
				return LogLevel::DEBUG;
			case websocketpp::log::alevel::connect:
			case websocketpp::log::alevel::disconnect:
			case websocketpp::log::alevel::fail:
				return LogLevel::INFO;
		}
		return LogLevel::NONE;
	}

	LogLevel mapErrorLevelToLogLevel(websocketpp::log::level level) const
	{
		switch (level) {
			case websocketpp::log::elevel::devel:
				return LogLevel::DEBUG;
			case websocketpp::log::elevel::info:
				return LogLevel::INFO;
			case websocketpp::log::elevel::warn:
				return LogLevel::WARN;
			case websocketpp::log::elevel::rerror:
			case websocketpp::log::elevel::fatal:
				return LogLevel::ERROR;
		}
		return LogLevel::NONE;
	}

	LogType type_ = LogType::ACCESS;
	LogLevel level_ = LogLevel::NONE;
	std::function<void(std::string)> debugLogger_ = [](std::string msg) {};
	std::function<void(std::string)> infoLogger_ = [](std::string msg) {};
	std::function<void(std::string)> warnLogger_ = [](std::string msg) {};
	std::function<void(std::string)> errorLogger_ = [](std::string msg) {};
};

}
}
