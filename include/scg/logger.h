#pragma once

// User-overridable logging macros
// Define these before including SCG headers to customize logging behavior
// By default, all logging is disabled (zero cost)

#ifndef SCG_LOG_DEBUG
#define SCG_LOG_DEBUG(msg) ((void)0)
#endif

#ifndef SCG_LOG_INFO
#define SCG_LOG_INFO(msg) ((void)0)
#endif

#ifndef SCG_LOG_WARN
#define SCG_LOG_WARN(msg) ((void)0)
#endif

#ifndef SCG_LOG_ERROR
#define SCG_LOG_ERROR(msg) ((void)0)
#endif

// Example usage (user can define before including headers):
// #define SCG_LOG_INFO(msg) std::cout << "[INFO] " << msg << std::endl
// #define SCG_LOG_ERROR(msg) std::cerr << "[ERROR] " << msg << std::endl
