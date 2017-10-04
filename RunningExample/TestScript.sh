ETH_FOLDER=/tmp/ECFCheckerEthereum
#ETH_FOLDER=~/.ethereum
echo "TestScript ::: Erasing ethereum folder: ${ETH_FOLDER}"
rm -rf /tmp/ECFCheckerEthereum

echo "TestScript ::: Creating primary account..."
ADDRESS=`../build/bin/geth --datadir=${ETH_FOLDER} --password "password" account new | grep Address | awk '{print $2}' | cut -d "{" -f 2 | cut -d "}" -f 1`
echo "TestScript ::: Writing ${ADDRESS} to genesis file..."
sed "s/REPLACEME/${ADDRESS}/g" genesis_template.json > genesis.json

echo "TestScript ::: Creating secondary account..."
../build/bin/geth --datadir=${ETH_FOLDER} --password "password" account new >& /dev/null

echo "TestScript ::: Initializing ethereum network..."
../build/bin/geth --datadir=${ETH_FOLDER} init genesis.json  >& /dev/null

echo "TestScript ::: Deploying and attacking the non-ECF contract:"
../build/bin/geth --datadir=${ETH_FOLDER} --unlock 0 --password "password" --verbosity 0 js ecfcheck.js 



