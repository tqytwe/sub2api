const fs = require('fs');

// P0-2: Arena 奖励机制说明的国际化
const arenaI18nZh = `
    arena: {
      rewardMechanism: {
        title: '奖励机制说明',
        description: '农场代币仅用于排名竞争，不能直接兑换。最终奖励按榜单排名发放固定金额。',
        monthlyTitle: '月榜奖励档位',
        dailyTitle: '日榜奖励档位',
        rankLabel: '第 {rank} 名',
        amountLabel: '{amount}',
        topRanks: 'Top {count}',
        settlementNote: '奖励在结算期结束后自动发放到账户余额'
      }
    },`;

const arenaI18nEn = `
    arena: {
      rewardMechanism: {
        title: 'Reward Mechanism',
        description: 'Arena tokens are only for ranking competition and cannot be exchanged directly. Final rewards are fixed amounts distributed by leaderboard rank.',
        monthlyTitle: 'Monthly Leaderboard Rewards',
        dailyTitle: 'Daily Leaderboard Rewards',
        rankLabel: 'Rank {rank}',
        amountLabel: '{amount}',
        topRanks: 'Top {count}',
        settlementNote: 'Rewards are automatically credited to your balance after the settlement period ends'
      }
    },`;

// P0-3: 盲盒奖励分支说明的国际化
const blindboxI18nZh = `
    blindbox: {
      rewardBranches: {
        title: '奖励类型',
        description: '开盒后将随机获得以下奖励之一：',
        coupon: '优惠券',
        redeemCode: '兑换码',
        balance: '余额',
        probability: '概率 {percent}%',
        note: '具体奖励金额/折扣根据当前奖池配置随机生成'
      }
    }`;

const blindboxI18nEn = `
    blindbox: {
      rewardBranches: {
        title: 'Reward Types',
        description: 'You will randomly receive one of the following rewards:',
        coupon: 'Coupon',
        redeemCode: 'Redeem Code',
        balance: 'Balance',
        probability: 'Probability {percent}%',
        note: 'Specific reward amounts/discounts are randomly generated based on the current pool configuration'
      }
    }`;

console.log('Arena 中文国际化:');
console.log(arenaI18nZh);
console.log('\nArena 英文国际化:');
console.log(arenaI18nEn);
console.log('\n盲盒中文国际化:');
console.log(blindboxI18nZh);
console.log('\n盲盒英文国际化:');
console.log(blindboxI18nEn);

console.log('\n✅ 国际化内容已准备完成，请手动添加到对应文件的 play 对象中');
