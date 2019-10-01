set key autotitle columnhead
set logscale xy
set logscale zcb

# set title "Channel Distribution vs Capacity and Fee Rate"
# set xlabel "Capacity (sat)"
# set ylabel "Fee Rate (bps)"
# set zlabel "Num Channels" offset 0,7

set terminal pngcairo size 800,600 enhanced font 'Verdana,10'
set output 'feelocus.png'

vsize = 150
feerate1(x) = 1 * 1e4 * vsize / x
feerate10(x) = 10 * 1e4 * vsize / x
feerate100(x) = 100 * 1e4 * vsize / x

set xrange [100:1e9]
set yrange [0.01:1000]
plot feerate1(x) with lines, feerate10(x) with lines, feerate100(x) with lines
