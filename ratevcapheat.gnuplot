set key autotitle columnhead
set logscale xy
set logscale zcb
set hidden3d
set dgrid3d 32,27 qnorm 10

# set yrange [] reverse

set title "Channel Distribution vs Capacity and Fee Rate"
set xlabel "Capacity (sat)"
set ylabel "Fee Rate (bps)"
set zlabel "Num Channels" offset 0,7

# set xtics border offset -1.5,-1.0
# set ytics border offset 2.5,-0.5

set palette defined (0 0 0 0.5, 1 0 0 1, 2 0 0.5 1, 3 0 1 1, 4 0.5 1 0.5, 5 1 1 0, 6 1 0.5 0, 7 1 0 0, 8 0.5 0 0)

set terminal pngcairo size 800,600 enhanced font 'Verdana,10'
set output 'ratevcapheat.png'

#splot 'ratevcap.csv' using  1:2:($3+1) '%lf,%lf,%lf' with lines palette

set pm3d map explicit interpolate 10,10
splot 'ratevcap.csv' using 1:2:($3+1) '%lf,%lf,%lf' with pm3d
