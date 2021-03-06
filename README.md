

LND Tool
----------------------------------------------------------------

The lndtool is a collection of utilities that might be useful for
maintaining lnd lightning nodes. The tool connects to an operating lnd
server via the gRPC port and requires an admin macaroon.


#### Channel List

The channel list is useful for examining the channel state of an lnd
node.  It shows per-channel balance and forwarding statistics as well
as infomation about the connected peer.


```
             ChanId Flg  Capacity     Local    Remote  Imbalance FwdR  FwdS  PubKey                                                              Log Alias
 632413799656325121 LAE   2000000    989939    993546      -1803 0     4.4e4 02809e936f0e82dfce13bcc47c77112db068f569e1db29e7bf98bcdd68b838ee84  8.8 powernode.io
 632414899113623553 LAE   1000000    491695    491790        -47 0     4.5e4 02ad6fb8d693dc1e4569bcedefadf5f72a931ae027dc0f0c544b34c1c6f3b9a02b  9.0 rompert.com🔵
 632414899113689088 LAE   1000000    491741    491744         -1 0     7.8e3 0331f80652fb840239df8dc99205792bba2e559a05469915804c08420230e23c7c  9.4 LightningPowerUsers.com
 632418197617311744 LAE   1000000    409960    573525     -81782 0     1.5e5 02529db69fd2ebd3126fb66fafa234fc3544477a23d509fe93ed229bb0e92e4fb8  9.0 🚀🌑 BOLTENING.club
 632418197617377280 LAE   1000000    491695    491790        -47 0     0     0279c22ed7a068d10dc1a38ae66d2d6461e269226c60258c021b1ddcdfe4b00bc4  9.4 ln1.satoshilabs.com
 632420396738805761 LAE   1000000     83487    899999    -408256 0     4.0e5 0232e20e7b68b9b673fb25f48322b151a93186bffe4550045040673797ceca43cf  9.0 zigzag.io
 632423695218835457 LAE   1000000    489134    494351      -2608 0     2.0e4 02bb24da3d0fb0793f4918c7599f973cc402f0912ec3fb530470f1fc08bdd6ecb5  9.4 LNBIG.com [lnd-10]
 632423695223422977 LAE   1000000    490642    492843      -1100 0     0     02de11c748f5b25cfd2ce801176d3926bfde4de23b1ff43e692a5b76cf06805e4a  9.4 LNBIG.com [lnd-09]
 632423695223488512 LAE   1000000     11651    971834    -480091 0     0     034ea80f8b148c750463546bd999bf7321a0e6dfc60aaf84bd0400a2e8d376c0d5  9.5 LNBIG.com [lnd-12]
 632424794675675136 LAE   1000000    489131    494354      -2611 0     0     033e9ce4e8f0e68f7db49ffb6b9eecc10605f3f3fcb3c630545887749ab515b9c7  9.4 LNBIG.com [lnd-11]
 632424794675740673 LAE   1000000    488341    494353      -3006 0     0     035f5236d7e6c6d16107c1f86e4514e6ccdd6b2c13c2abc1d7a83cd26ecb4c1d0e  9.4 LNBIG.com [lnd-13]
 632436889353846785 LAE    384993    184191    184287        -48 0     0     0217890e3aad8d35bc054f43acc00084b25229ecff0ab68debd82883ad65ee8266  8.9 1ML.com node ALPHA
 632588622058618880 RAE     50000     18545     14940       1803 0     0     0232fe448d6f8e9e8e54394f3dc5b35013b7a3a3cd227ffce1bb81cc8d285cf0a5  6.9 spookiestevie
 632616109729185792 RIE  16777210   8383999   8381574       1213 2.4e6 0     022939ab7046b11f67bf04f9f1d66058a0d2089a8273f3d0f8eb125c2911e6503e  7.2 
 632868997415763968 RAE   4000000   1721871   2261615    -269872 0     0     0311cad0edf4ac67298805cf4407d94358ca60cd44f2e360856f3b1c088bcd4782  9.4 LNBIG.com [lnd-42]
 632875594501849089 LAE  16777215   7109240   9651460   -1271110 0     3.9e6 03864ef025fde8fb587d989186ce6a4a186895ee44a926bfc370e2c366597a3f8f  9.6 ACINQ
 632877793458323457 LAE  16777215   8379248   8381452      -1102 2.2e5 2.4e6 0395033b252c6f40e3756984162d68174e2bd8060a129c0d3462a9370471c6d28f  8.7 BitMEXResearch
 634227993793855489 RAE   8000000   3991699   3991785        -43 4.4e6 0     030c3f19d742ca294a55c00376b3b355c3c90d61c6b6b39554dbc7ac19b141c14f  9.4 Bitrefill.com
 634235690383900673 LAE   8000000   3990928   3991792       -432 0     0     023c5b5667b16cd7fcca5591a8c0f47beb76c9405e16a4f2d6b42c7b9904a7f0e6  7.9 OpinionatedGeek ⚡
 634235690383966209 LAE   8000000   3989938   3993547      -1804 0     0     021c97a90a411ff2b10dc2a8e32de2f29d2fa49d41bfbb52bd416e460db0747d0d  8.4 021c97a90a411ff2b10d
 634235690384097281 LAE   8000000   3989939   3993546      -1803 0     4.9e5 02e9046555a9665145b0dbd7f135744598418df7d61d3660659641886ef1274844  9.2 SilentBob
 634248884468318209 LAE   8000000   3989253   3994232      -2489 0     0     0233502f6370758ede277d0ee5308a900feffeea29dbe9fc9593d1d0c30b2eb30e  9.0 Abacus Routing [BB2]
 634495175203487745 LAE   4000000   1992332   1991153        590 1.0e6 0     0202f05149350a1c68578238eab17c594d1f5bd5235864c413c50484b98b2f32e5  7.2 Fran
 634594131205292032 LID   8000000   7449186    535864    3456661 5.1e6 0     02c1d012a6caf8ea783ef5a3d77ccc98d0de9137aac69c92c8dfd3ac3f3043b28f  7.9 dapplnd.com
 634759057927700481 LID   8000000   3992135   3992844       -354 0     0     023bc9c7b5680113c6a984de929a2a425f7c15be141f41c5efef6d03f22d7379d0  7.3 AtomicAlpha
 634807436449808385 LAE   8000000   7847905    135580    3856163 0     0     035ef1c0ef3c3273820abeb6136a3c79736d7af4e1cb8783410eb022b5a46390a3  7.9 “quitebeyond”
 634821730102804480 LAE    200000     89938     93547      -1804 0     0     0232fe448d6f8e9e8e54394f3dc5b35013b7a3a3cd227ffce1bb81cc8d285cf0a5  6.9 spookiestevie
 635057025564344321 LAE   8000000   7542066    441419    3550324 1.2e6 0     0234d70cf65a4e8974d3d49e23ec7b9792877f783f2f5dfd3882422aada18a916c  7.7 intgrs
 635765111093329921 RAE   2644164   1312525   1315124      -1299 0     0     0390b5d4492dc2f5318e5233ab2cebf6d48914881a33ef6a9c6bcdbb433ad986d0  9.5 LNBIG.com [lnd-01]
 635807992037638145 LAE   4000000   1991695   1991790        -47 1.3e4 0     03317598ee590de12b4e267982ba2b3250cc6059549203b6d658b29ace439c63fc  7.7 raito.systemb.co
 635809091447357441 LAE   8000000   7636839    346646    3645097 2.1e5 0     03c07a1f01a7a5c5ee6c6c63c27df976bc1fbaac09bbee047dad2e5c338c5abae8  8.0 lnd-1.chaintools.io
 636241199610265601 LAE   8000000   3613414   4370071    -378328 0     6.1e6 02a04446caa81636d60d63b066f2814cbd3a6b5c258e3172cbdded7a16e2cfff4c  8.8 ln.bitstamp.net [Bitstamp]
 636453405334503425 RAE   4000000   1713247   2269474    -278113 0     0     03e5ea100e6b1ef3959f79627cb575606b19071235c48b3e7f9808ebcd6d12e87d  9.4 LNBIG.com [lnd-31]
 636928394378346496 RAE   4000000   1459730   2523755    -532012 0     8.9e5 028a8e53d70bc0eb7b5660943582f10b7fd6c727a78ad819ba8d45d6a638432c49  9.4 LNBIG.com [lnd-33]
 637252750309654528 RAE   5000000   2084506   2898980    -407237 0     0     03d06758583bb5154774a6eb221b1276c9e82d65bbaceca806d90e20c108f4b1c7  9.1 yalls.org
 637522130689851393 LAE   4000000   1989334   1994199      -2432 0     4.9e4 0237fefbe8626bf888de0cad8c73630e32746a22a2c4faa91c1d9877a3826e1174  8.6 1.ln.aantonop.com
 637569409742143488 RAE   5000000     10000   4973486   -2481743 0     0     03d06758583bb5154774a6eb221b1276c9e82d65bbaceca806d90e20c108f4b1c7  9.1 yalls.org
 638094976164626433 RAE   1000000    490888    491734       -423 1.5e3 1.5e3 024794d1446e510c75f84bbd75cc151124522aed03ad75a20f1708f77e5f3e674a  7.3 LogansRun
 638112568423088129 RAE   2200000         0   2182721   -1091360 0     0     03a5886df676f3b3216a4520156157b8d653e262b520281d8d325c24fd8b456b9c  8.1 Pinky
 638333570219573249 RAE    191292         0    174012     -87006 0     0     032895e3187c376c65c143cad3660a8b43ef607d1dc6029573a24de15a91810357  7.0 SchneeFlocke
 639339623315931137 RIE    109533         0     94520     -47260 0     0     026a310e6ffd8ea0769eb137ac70e55fc556db17d53bdd826f5dbbd2f31825d39f  5.0 026a310e6ffd8ea0769e
41                      191111622 101892007  88547278    6672365 1.4e7 1.4e7 02a5fa844d310f582d209fe649352b225440b8a54e77361f229bb92ee263c87e6f  8.3 BonsaiSoftware
```

#### Rebalance

The rebalance subcommand uses a loop route to send funds from a
specified "source" channel to a specified "destination" channel.  This
is useful to reduce channel imbalances.

A sample rebalance command looks like:
```
lndtool rebalance -a 1000000 -s 635057025564344321 -d 637569409742143488
```

#### Recommend

The recommend subcommand evaulates overall channel state and
constructs a rebalance command which will improve the overall balance
of the node.

If the `--doit` flag is asserted the rebalance command will be
directly executed instead of printed.

#### Autobalance

The autobalance command loops using the recommend command and
executing repeated rebalance commands.  It maintains a local sqlite
database to avoid retrying rebalance pairs which don't find a
successful route.

#### Farside

The farside subcommand extracts a channel graph of the network and
attempts to determine well-funded, well-connected nodes that are
"distant" (by hops and/or fees) and would therefore be attractive
targets for new channels.  The farside subcommand is under development
and is not currently very reliable.

#### Usage

```
[user@bonsai lndtool]$ ./lndtool --help
Usage:
  lndtool [OPTIONS] <command>

Application Options:
      --verbose                      Verbose output
      --network=                     Network (mainnet, testnet, ...) (default: mainnet)
      --lnddir=                      The base directory that contains lnd's data, logs, configuration file, etc. (default: /home/user/.lnd)
      --lndtooldir=                  The base directory that contains lndtool's data, logs, configuration file, etc. (default: /home/user/.lndtool)
      --configfile=                  Path to configuration file (default: /home/user/.lndtool/lndtool-mainnet.conf)
      --dbfile=                      Path to database file (default: /home/user/.lndtool/lndtool-mainnet.db)
      --tlscertpath=                 Path to read the TLS certificate for lnd's RPC and REST services (default: /home/user/.lnd/tls.cert)
      --macaroonpath=                path to macaroon file (default: /home/user/.lnd/data/chain/bitcoin/mainnet/admin.macaroon)
      --rpcserver=                   host:port of ln daemon (default: localhost:10009)

Channels:
      --channels.statswindow=        Time window for channel statistics (default: 720h0m0s)

Rebalance:
      --rebalance.finalcltvdelta=    Final CLTV delta (default: 144)
      --rebalance.feelimitrate=      Limit fees to this rate (default: 0.0005)

Recommend:
      --recommend.srcchantarget=     Adds channel to source target list (default: all)
      --recommend.dstchantarget=     Adds channel to destination target list (default: all)
      --recommend.peernodeblacklist= Adds node to peers to skip
      --recommend.minimbalance=      Minimum imbalance to consider rebalancing (default: 1000)
      --recommend.transferamount=    Size of rebalance transfers (default: 10000)
      --recommend.retryinhibit=      Inhibit retrying failed loops for this long (default: 1h0m0s)

Help Options:
  -h, --help                         Show this help message

Available commands:
  autobalance  Loop balancing channels
  channels     Lists channels in tabular form
  dumpconfig   Dumps the configuration to stdout
  farside      Finds nodes on the far side of the connected set
  rebalance    Balance a pair of channels with a loop transaction
  recommend    Recommend a pair of channels to rebalance

```
