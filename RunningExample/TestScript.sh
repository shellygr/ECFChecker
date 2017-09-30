ETH_FOLDER=/tmp/ECFCheckerEthereum
#ETH_FOLDER=~/.ethereum
echo "Erasing ethereum folder: ${ETH_FOLDER}"
rm -rf /tmp/ECFCheckerEthereum

echo "Creating primary account..."
ADDRESS=`../build/bin/geth --datadir=${ETH_FOLDER} --password "password" account new | grep Address | awk '{print $2}' | cut -d "{" -f 2 | cut -d "}" -f 1`
echo "Writing ${ADDRESS} to genesis file..."
sed "s/REPLACEME/${ADDRESS}/g" genesis_template.json > genesis.json

echo "Creating secondary account..."
../build/bin/geth --datadir=${ETH_FOLDER} --password "password" account new >& /dev/null

echo "Initializing ethereum network..."
../build/bin/geth --datadir=${ETH_FOLDER} init genesis.json  >& /dev/null

echo "Deploying and attacking the non-ECF contract:"
../build/bin/geth --datadir=${ETH_FOLDER} --unlock 0 --password "password" js ecfcheck.js 



