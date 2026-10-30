# Examples

This directory contains runnable Go examples for the Alpaca SDK. Examples that can place or cancel orders explicitly connect to the paper-trading API and require paper-account credentials in the `APCA_API_KEY_ID` and `APCA_API_SECRET_KEY` environment variables. Do not paste credentials into source files. Regular paper accounts use the standard `data.alpaca.markets` host for Market Data, so a production Market Data hostname does not mean that an example can place live orders. These examples are educational and are not intended for unattended or live trading. Their strategies are not guaranteed to perform well or to be suitable for your account.

## Mean Reversion

This trading algorithm bases its strategy on a mean reversion theory, which essentially guesses that the stock price will correct to the mean.  This means we'd want to execute trades when the stock price is below the running average, as the theory states that the stock price will eventually rise to the mean.  The algorithm does this by taking the 20 minute running average stock price of a given stock (in this case "AAPL") and longs or sells based on the average.  After every minute, the algorithm will re-evaluate the mean and see if adjustments to the position need to be made.  For more information on this strategy, you can read [this link](https://medium.com/automation-generation/a-simple-mean-reversion-stock-trading-script-in-c-fdd3d147af95) detailing a mean reversion strategy in C#.

## Long-Short Equity

This trading algorithm implements the long-short equity strategy.  This means that the algorithm will rank a given universe of stocks based on a certain metric, and long the top ranked stocks and short the lower ranked stocks.  More specifically, the algorithm uses the frequently used 130/30 percent equity split between longs and shorts (130% of equity used for longs, 30% of equity used for shorts).  The algorithm will then grab the top and bottom 25% of stocks, and long or short them accordingly.  The algorithm will purchase equal quantities across a bucket of stocks, so all stocks in the long bucket are ordered with the same quantity (same with the short bucket).  After every minute, the algorithm will re-rank the stocks and make adjustments to the position if necessary.  For more information on this strategy, read this link [here](https://www.investopedia.com/terms/l/long-shortequity.asp).

Some stocks cannot be shorted.  In this case, the algorithm uses the leftover equity from the stocks that could not be shorted and shorts the stocks have already been shorted.

The algorithm uses percent change in stock price over the past 10 minutes to rank the stocks, where the stocks that rose the most are longed and the ones that sunk the most are shorted.