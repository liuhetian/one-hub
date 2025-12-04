#!/bin/bash

# HTTP性能压测脚本
# 使用方法: ./benchmark.sh [URL] [并发数] [持续时间] [API_KEY]

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 配置参数
URL="${1:-http://127.0.0.1:8080/v1/chat/completions}"
CONCURRENCY="${2:-200}"
DURATION="${3:-30}"
API_KEY="${4:-${API_KEY:-test-key}}"
REQUESTS=$((CONCURRENCY * 1000))

# 请求数据
REQUEST_DATA='{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "developer",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hello!"
    }
  ]
}'

echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}        HTTP性能压测工具 - OpenAI Mock Service${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${YELLOW}测试配置:${NC}"
echo -e "  目标URL:    ${GREEN}${URL}${NC}"
echo -e "  并发数:     ${GREEN}${CONCURRENCY}${NC}"
echo -e "  测试时长:   ${GREEN}${DURATION}秒${NC}"
echo -e "  总请求数:   ${GREEN}${REQUESTS}${NC}"
echo -e "  API KEY:    ${GREEN}${API_KEY:0:20}...${NC}"
echo ""

# 创建临时文件用于存储请求数据
TEMP_DIR=$(mktemp -d)
trap "rm -rf ${TEMP_DIR}" EXIT
REQUEST_FILE="${TEMP_DIR}/request.json"
echo "${REQUEST_DATA}" > "${REQUEST_FILE}"

# 检查URL是否可访问
echo -e "${BLUE}[1/4] 检查目标服务...${NC}"
if curl -s --connect-timeout 5 \
    -X POST "${URL}" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${API_KEY}" \
    -d "${REQUEST_DATA}" > /dev/null 2>&1; then
    echo -e "${GREEN}✓ 服务可访问且响应正常${NC}"
else
    echo -e "${RED}✗ 无法访问目标URL或请求失败: ${URL}${NC}"
    echo -e "${YELLOW}提示: 请确保服务已启动${NC}"
    exit 1
fi
echo ""

# 检查工具是否安装
echo -e "${BLUE}[2/4] 检查压测工具...${NC}"
TOOLS_AVAILABLE=0

if command -v wrk &> /dev/null; then
    echo -e "${GREEN}✓ wrk 已安装${NC}"
    TOOLS_AVAILABLE=$((TOOLS_AVAILABLE + 1))
else
    echo -e "${YELLOW}✗ wrk 未安装 (可选)${NC}"
fi

if command -v hey &> /dev/null; then
    echo -e "${GREEN}✓ hey 已安装${NC}"
    TOOLS_AVAILABLE=$((TOOLS_AVAILABLE + 1))
else
    echo -e "${YELLOW}✗ hey 未安装 (可选)${NC}"
fi

if command -v ab &> /dev/null; then
    echo -e "${GREEN}✓ ab 已安装${NC}"
    TOOLS_AVAILABLE=$((TOOLS_AVAILABLE + 1))
else
    echo -e "${YELLOW}✗ ab 未安装 (可选)${NC}"
fi

if [ $TOOLS_AVAILABLE -eq 0 ]; then
    echo -e "${RED}错误: 没有找到任何压测工具${NC}"
    echo -e "${YELLOW}请安装: brew install wrk hey${NC}"
    exit 1
fi
echo ""

# 创建结果目录
RESULT_DIR="benchmark_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
mkdir -p "${RESULT_DIR}"

echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}                    开始性能测试${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""

# 检查并提示文件句柄限制
CURRENT_ULIMIT=$(ulimit -n 2>/dev/null || echo "256")
if [ "${CURRENT_ULIMIT}" -lt 10000 ]; then
    echo -e "${YELLOW}⚠ 文件句柄限制较低: ${CURRENT_ULIMIT}${NC}"
    echo -e "${YELLOW}  建议执行: ulimit -n 65535${NC}"
    echo ""
fi

# WRK 测试
if command -v wrk &> /dev/null; then
    echo -e "${BLUE}[3/4] 运行 wrk 压测...${NC}"
    echo -e "${YELLOW}配置: ${DURATION}秒, ${CONCURRENCY}并发, 8线程${NC}"
    echo ""

    # 创建 wrk Lua 脚本
    WRK_SCRIPT="${TEMP_DIR}/wrk.lua"
    cat > "${WRK_SCRIPT}" << 'EOF'
wrk.method = "POST"
wrk.headers["Content-Type"] = "application/json"
wrk.headers["Authorization"] = "Bearer " .. os.getenv("WRK_API_KEY")
wrk.body = [[{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "developer",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hello!"
    }
  ]
}]]
EOF

    WRK_OUTPUT="${RESULT_DIR}/wrk_${TIMESTAMP}.txt"
    WRK_API_KEY="${API_KEY}" wrk -t8 -c"${CONCURRENCY}" -d"${DURATION}s" --latency -s "${WRK_SCRIPT}" "${URL}" | tee "${WRK_OUTPUT}"

    echo ""
    echo -e "${GREEN}✓ wrk 测试完成，结果已保存到: ${WRK_OUTPUT}${NC}"
    echo ""
    echo -e "${YELLOW}等待 10 秒让端口释放...${NC}"
    sleep 10
fi

# HEY 测试
if command -v hey &> /dev/null; then
    echo -e "${BLUE}[3/4] 运行 hey 压测...${NC}"
    echo -e "${YELLOW}配置: ${DURATION}秒, ${CONCURRENCY}并发${NC}"
    echo ""

    HEY_OUTPUT="${RESULT_DIR}/hey_${TIMESTAMP}.txt"
    # -z 默认最多 1M 请求，需指定更大的 -n 突破限制
    hey -z "${DURATION}s" -n 100000000 -c "${CONCURRENCY}" \
        -cpus 10 \
        -t 0 \
        -disable-compression \
        -m POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${API_KEY}" \
        -d "${REQUEST_DATA}" \
        "${URL}" | tee "${HEY_OUTPUT}"

    echo ""
    echo -e "${GREEN}✓ hey 测试完成，结果已保存到: ${HEY_OUTPUT}${NC}"
    echo ""
fi

# AB 测试
if command -v ab &> /dev/null; then
    echo -e "${BLUE}[3/4] 运行 ab 压测...${NC}"

    # macOS 上端口耗尽问题：等待 TIME_WAIT 连接释放
    echo -e "${YELLOW}等待 15 秒让端口释放（避免 macOS 端口耗尽）...${NC}"
    sleep 15

    echo -e "${YELLOW}配置: ${DURATION}秒, ${CONCURRENCY}并发, Keep-Alive${NC}"
    echo ""

    # ab 在 macOS 上对 localhost 有兼容性问题，需要替换为 127.0.0.1
    AB_URL="${URL//localhost/127.0.0.1}"

    AB_OUTPUT="${RESULT_DIR}/ab_${TIMESTAMP}.txt"
    # -t 隐含 -n 50000，需要指定更大的 -n 确保测试运行完整时长
    ab -t "${DURATION}" -n 10000000 -c "${CONCURRENCY}" -k \
        -p "${REQUEST_FILE}" \
        -T "application/json" \
        -H "Authorization: Bearer ${API_KEY}" \
        "${AB_URL}" 2>&1 | tee "${AB_OUTPUT}"

    echo ""
    echo -e "${GREEN}✓ ab 测试完成，结果已保存到: ${AB_OUTPUT}${NC}"
    echo ""
fi

# 生成汇总报告
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}                    测试结果汇总${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""

SUMMARY_FILE="${RESULT_DIR}/summary_${TIMESTAMP}.txt"

{
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║              HTTP 性能压测报告                                 ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""
    echo "测试时间: $(date '+%Y-%m-%d %H:%M:%S')"
    echo "目标URL:  ${URL}"
    echo "并发数:   ${CONCURRENCY}"
    echo "测试时长: ${DURATION}秒"
    echo "总请求:   ${REQUESTS}"
    echo "API KEY:  ${API_KEY:0:20}..."
    echo "请求格式: POST + JSON Body + Authorization Header"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    if [ -f "${WRK_OUTPUT}" ]; then
        echo "【wrk 测试结果】"
        echo ""

        # 提取关键指标
        WRK_QPS=$(grep "Requests/sec:" "${WRK_OUTPUT}" | awk '{print $2}')
        WRK_THROUGHPUT=$(grep "Transfer/sec:" "${WRK_OUTPUT}" | awk '{print $2}')
        WRK_TOTAL=$(grep "requests in" "${WRK_OUTPUT}" | awk '{print $1}')
        WRK_DURATION=$(grep "requests in" "${WRK_OUTPUT}" | awk '{print $4}')
        WRK_AVG=$(grep "Latency" "${WRK_OUTPUT}" | head -1 | awk '{print $2}')
        WRK_MAX=$(grep "Latency" "${WRK_OUTPUT}" | head -1 | awk '{print $4}')

        echo "  吞吐量指标:"
        echo "    ├─ QPS:           ${WRK_QPS} req/s"
        echo "    ├─ 传输速率:       ${WRK_THROUGHPUT}"
        echo "    ├─ 总请求数:       ${WRK_TOTAL}"
        echo "    └─ 测试时长:       ${WRK_DURATION}"
        echo ""

        echo "  延迟统计:"
        echo "    ├─ 平均延迟:       ${WRK_AVG}"
        echo "    └─ 最大延迟:       ${WRK_MAX}"
        echo ""

        # 提取百分位延迟
        echo "  延迟分布 (百分位):"
        grep -A 4 "Latency Distribution" "${WRK_OUTPUT}" | tail -4 | while read -r line; do
            if [[ $line =~ ^[[:space:]]*([0-9]+)%[[:space:]]+([0-9.]+[a-z]+) ]]; then
                pct="${BASH_REMATCH[1]}"
                val="${BASH_REMATCH[2]}"
                echo "    ├─ P${pct}:            ${val}"
            fi
        done | sed '$ s/├/└/'
        echo ""

        # 错误统计
        WRK_ERRORS=$(grep "Non-2xx or 3xx responses:" "${WRK_OUTPUT}" | awk '{print $NF}')
        if [ -n "${WRK_ERRORS}" ] && [ "${WRK_ERRORS}" != "0" ]; then
            WRK_ERROR_RATE=$(echo "scale=2; ${WRK_ERRORS} * 100 / ${WRK_TOTAL}" | bc)
            echo "  错误统计:"
            echo "    ├─ 非2xx/3xx响应:  ${WRK_ERRORS}"
            echo "    └─ 错误率:         ${WRK_ERROR_RATE}%"
            echo ""
        fi
    fi

    if [ -f "${HEY_OUTPUT}" ]; then
        echo "【hey 测试结果】"
        echo ""

        # 提取关键指标
        HEY_QPS=$(grep "Requests/sec:" "${HEY_OUTPUT}" | awk '{print $2}')
        HEY_TOTAL_TIME=$(grep "Total:" "${HEY_OUTPUT}" | awk '{print $2, $3}')
        HEY_AVG=$(grep "Average:" "${HEY_OUTPUT}" | awk '{print $2, $3}')
        HEY_FASTEST=$(grep "Fastest:" "${HEY_OUTPUT}" | awk '{print $2, $3}')
        HEY_SLOWEST=$(grep "Slowest:" "${HEY_OUTPUT}" | awk '{print $2, $3}')
        HEY_TOTAL_REQS=$(grep -A 10 "Status code distribution:" "${HEY_OUTPUT}" | grep -E "^\s*\[" | awk '{sum+=$2} END {print sum}')

        echo "  吞吐量指标:"
        echo "    ├─ QPS:           ${HEY_QPS} req/s"
        echo "    ├─ 总耗时:         ${HEY_TOTAL_TIME}"
        echo "    └─ 总请求数:       ${HEY_TOTAL_REQS}"
        echo ""

        echo "  延迟统计:"
        echo "    ├─ 平均延迟:       ${HEY_AVG}"
        echo "    ├─ 最快请求:       ${HEY_FASTEST}"
        echo "    └─ 最慢请求:       ${HEY_SLOWEST}"
        echo ""

        # 提取百分位延迟
        echo "  延迟分布 (百分位):"
        grep -A 7 "Latency distribution:" "${HEY_OUTPUT}" | tail -7 | while read -r line; do
            if [[ $line =~ ([0-9]+)%[[:space:]]+in[[:space:]]+([0-9.]+)[[:space:]]+secs ]]; then
                pct="${BASH_REMATCH[1]}"
                val="${BASH_REMATCH[2]}"
                echo "    ├─ P${pct}:            ${val}s"
            fi
        done | sed '$ s/├/└/'
        echo ""

        # 状态码分布
        echo "  响应状态:"
        HEY_STATUS_LINES=$(grep -A 10 "Status code distribution:" "${HEY_OUTPUT}" | grep -E "^\s*\[" | wc -l | tr -d ' ')
        HEY_LINE_NUM=0
        grep -A 10 "Status code distribution:" "${HEY_OUTPUT}" | grep -E "^\s*\[" | while read -r line; do
            HEY_LINE_NUM=$((HEY_LINE_NUM + 1))
            code=$(echo "$line" | grep -oE '\[[0-9]+\]')
            count=$(echo "$line" | awk '{print $2}')
            if [ "${HEY_LINE_NUM}" -eq "${HEY_STATUS_LINES}" ]; then
                echo "    └─ ${code}  ${count} 响应"
            else
                echo "    ├─ ${code}  ${count} 响应"
            fi
        done
        echo ""
    fi

    if [ -f "${AB_OUTPUT}" ]; then
        echo "【ab 测试结果】"
        echo ""

        # 提取关键指标
        AB_QPS=$(grep "Requests per second:" "${AB_OUTPUT}" | awk '{print $4}')
        AB_TIME=$(grep "Time per request:" "${AB_OUTPUT}" | head -1 | awk '{print $4, $5}')
        AB_COMPLETE=$(grep "Complete requests:" "${AB_OUTPUT}" | awk '{print $3}')
        AB_FAILED=$(grep "Failed requests:" "${AB_OUTPUT}" | awk '{print $3}')
        AB_TRANSFER=$(grep "Transfer rate:" "${AB_OUTPUT}" | awk '{print $3, $4}')

        echo "  吞吐量指标:"
        echo "    ├─ QPS:           ${AB_QPS} req/s"
        echo "    ├─ 传输速率:       ${AB_TRANSFER}"
        echo "    ├─ 完成请求:       ${AB_COMPLETE}"
        echo "    └─ 失败请求:       ${AB_FAILED}"
        echo ""

        echo "  延迟统计:"
        echo "    └─ 平均延迟:       ${AB_TIME}"
        echo ""

        # 提取百分位延迟
        echo "  延迟分布 (百分位):"
        grep -A 9 "Percentage of the requests served" "${AB_OUTPUT}" | tail -8 | while read -r line; do
            if [[ $line =~ ^[[:space:]]*([0-9]+)%[[:space:]]+([0-9]+) ]]; then
                pct="${BASH_REMATCH[1]}"
                val="${BASH_REMATCH[2]}"
                echo "    ├─ P${pct}:            ${val}ms"
            fi
        done | sed '$ s/├/└/'
        echo ""
    fi

    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "【性能对比总结】"
    echo ""

    # 对比表格
    printf "%-12s | %-15s | %-15s | %-15s | %-10s\n" "工具" "QPS (req/s)" "平均延迟" "P99 延迟" "错误率"
    echo "-------------|-----------------|-----------------|------------------|------------"

    if [ -f "${WRK_OUTPUT}" ]; then
        WRK_QPS_NUM=$(grep "Requests/sec:" "${WRK_OUTPUT}" | awk '{printf "%.2f", $2}')
        WRK_AVG_NUM=$(grep "Latency" "${WRK_OUTPUT}" | head -1 | awk '{print $2}')
        WRK_P99=$(grep "99%" "${WRK_OUTPUT}" | awk '{print $2}')
        WRK_TOTAL_NUM=$(grep "requests in" "${WRK_OUTPUT}" | awk '{print $1}')
        WRK_ERR_NUM=$(grep "Non-2xx or 3xx responses:" "${WRK_OUTPUT}" | awk '{print $NF}')
        if [ -n "${WRK_ERR_NUM}" ] && [ "${WRK_ERR_NUM}" != "0" ]; then
            WRK_ERR_RATE=$(echo "scale=2; ${WRK_ERR_NUM} * 100 / ${WRK_TOTAL_NUM}" | bc)%
        else
            WRK_ERR_RATE="0%"
        fi
        printf "%-12s | %15s | %15s | %16s | %10s\n" "wrk" "${WRK_QPS_NUM}" "${WRK_AVG_NUM}" "${WRK_P99}" "${WRK_ERR_RATE}"
    fi

    if [ -f "${HEY_OUTPUT}" ]; then
        HEY_QPS_NUM=$(grep "Requests/sec:" "${HEY_OUTPUT}" | awk '{printf "%.2f", $2}')
        HEY_AVG_NUM=$(grep "Average:" "${HEY_OUTPUT}" | awk '{printf "%.4fs", $2}')
        HEY_P99=$(grep "99%" "${HEY_OUTPUT}" | grep -oE "[0-9]+\.[0-9]+" | awk '{printf "%.4fs", $1}')
        HEY_200=$(grep -A 10 "Status code distribution:" "${HEY_OUTPUT}" | grep -E "^\s*\[200\]" | awk '{print $2}')
        HEY_TOTAL_NUM=$(grep -A 10 "Status code distribution:" "${HEY_OUTPUT}" | grep -E "^\s*\[" | awk '{sum+=$2} END {print sum}')
        if [ -n "${HEY_200}" ] && [ -n "${HEY_TOTAL_NUM}" ] && [ "${HEY_TOTAL_NUM}" != "0" ]; then
            HEY_ERR_NUM=$((HEY_TOTAL_NUM - HEY_200))
            HEY_ERR_RATE=$(echo "scale=2; ${HEY_ERR_NUM} * 100 / ${HEY_TOTAL_NUM}" | bc)%
        else
            HEY_ERR_RATE="N/A"
        fi
        printf "%-12s | %15s | %15s | %16s | %10s\n" "hey" "${HEY_QPS_NUM}" "${HEY_AVG_NUM}" "${HEY_P99}" "${HEY_ERR_RATE}"
    fi

    if [ -f "${AB_OUTPUT}" ]; then
        AB_QPS_NUM=$(grep "Requests per second:" "${AB_OUTPUT}" | awk '{printf "%.2f", $4}')
        AB_AVG_NUM=$(grep "Time per request:" "${AB_OUTPUT}" | head -1 | awk '{printf "%sms", $4}')
        AB_P99=$(grep "99%" "${AB_OUTPUT}" | awk '{printf "%sms", $2}')
        AB_COMPLETE=$(grep "Complete requests:" "${AB_OUTPUT}" | awk '{print $3}')
        AB_FAILED=$(grep "Failed requests:" "${AB_OUTPUT}" | awk '{print $3}')
        if [ -n "${AB_FAILED}" ] && [ "${AB_FAILED}" != "0" ]; then
            AB_ERR_RATE=$(echo "scale=2; ${AB_FAILED} * 100 / ${AB_COMPLETE}" | bc)%
        else
            AB_ERR_RATE="0%"
        fi
        printf "%-12s | %15s | %15s | %16s | %10s\n" "ab" "${AB_QPS_NUM}" "${AB_AVG_NUM}" "${AB_P99}" "${AB_ERR_RATE}"
    fi

    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║  报告生成时间: $(date '+%Y-%m-%d %H:%M:%S')                              ║"
    echo "╚════════════════════════════════════════════════════════════════╝"

} | tee "${SUMMARY_FILE}"

echo -e "${GREEN}✓ 汇总报告已保存到: ${SUMMARY_FILE}${NC}"
echo ""

echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}            所有测试完成！${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${YELLOW}结果文件位置:${NC}"
echo -e "  ${RESULT_DIR}/"
ls -lh "${RESULT_DIR}/"*"${TIMESTAMP}"* 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}'
echo ""
