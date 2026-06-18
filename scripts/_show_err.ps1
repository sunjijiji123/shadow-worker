$log = Get-Content 'C:\Users\Administrator\code\1-ai\shadow-worker\client\cfg_log.txt'
# 找到 "CMake Error" 行,打印它和后面 5 行
$lines = $log -split "`n"
for ($i = 0; $i -lt $lines.Count; $i++) {
    if ($lines[$i] -match 'CMake Error|Qt6CoreMacros.cmake:1953') {
        Write-Output "--- match at line $i ---"
        $start = [Math]::Max(0, $i - 1)
        $end = [Math]::Min($lines.Count - 1, $i + 5)
        for ($j = $start; $j -le $end; $j++) {
            Write-Output $lines[$j]
        }
        Write-Output ""
    }
}
